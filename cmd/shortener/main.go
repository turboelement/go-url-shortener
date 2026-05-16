package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-url-shortener/internal/audit"
	"go-url-shortener/internal/config"
	"go-url-shortener/internal/profiler"
	"go-url-shortener/internal/repository"
	"go-url-shortener/internal/server"
	"go-url-shortener/internal/service"

	"go.uber.org/zap"
)

func main() {
	cfg := config.New()

	logger, err := zap.NewProduction() // or zap.NewDevelopment()
	if err != nil {
		log.Fatalf("cannot initialize zap logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Server configuration loaded",
		zap.String("ServerAddr", cfg.ServerAddr),
		zap.String("BaseURL", cfg.BaseURL),
	)

	if cfg.EnablePprof {
		p := profiler.New()
		p.Start()
		defer p.Close()
		logger.Info("pprof server started", zap.String("address", p.Addr()))
	}

	var repo repository.URLRepositoryInterface

	if cfg.DatabaseDSN != "" {
		logger.Info("Using Database")
		dbRepo, err := repository.NewPostgresRepository(cfg.DatabaseDSN)
		if err != nil {
			logger.Fatal("Error connecting to DB", zap.Error(err))
		}
		defer dbRepo.Close()
		repo = dbRepo
	} else if cfg.FileStoragePath != "" {
		logger.Info("Using file storage",
			zap.String("file_path", cfg.FileStoragePath),
		)
		repo = repository.NewFileURLRepository(cfg.FileStoragePath)
	} else {
		logger.Info("Using in-memory storage")
		repo = repository.NewURLRepository()
	}

	svc := service.NewShortenerService(repo)
	defer svc.Close()

	auditSubject := audit.NewSubject()
	if cfg.AuditFilePath != "" {
		fo, err := audit.NewFileObserver(cfg.AuditFilePath)
		if err != nil {
			logger.Fatal("failed to create file audit observer", zap.Error(err))
		}
		auditSubject.Register(fo)
		logger.Info("Using file audit", zap.String("file_path", cfg.AuditFilePath))
	}
	if cfg.AuditURL != "" {
		auditSubject.Register(audit.NewHTTPObserver(cfg.AuditURL))
		logger.Info("Using HTTP audit", zap.String("url", cfg.AuditURL))
	}
	defer auditSubject.Close()

	router := server.NewRouter(server.RouterDeps{
		BaseURL:      cfg.BaseURL,
		CookieSecret: cfg.CookieSecret,
		Logger:       logger,
		Repo:         repo,
		Svc:          svc,
		AuditSubject: auditSubject,
	})

	srv := &http.Server{
		Addr:         cfg.ServerAddr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	logger.Info("Server running at",
		zap.String("address", cfg.BaseURL),
	)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server starting failed", zap.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	logger.Info("Server shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown failed", zap.Error(err))
	}

	logger.Info("Server stopped")
}
