package main

import (
	"context"
	"crypto/tls"
	"fmt"
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
	"go-url-shortener/internal/tlsconfig"

	"go.uber.org/zap"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func formatBuildInfo(label, value string) string {
	if value == "" {
		value = "N/A"
	}
	return fmt.Sprintf("%s: %s", label, value)
}

func main() {
	fmt.Println(formatBuildInfo("Build version", buildVersion))
	fmt.Println(formatBuildInfo("Build date", buildDate))
	fmt.Println(formatBuildInfo("Build commit", buildCommit))

	cfg, err := config.New()
	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("cannot initialize zap logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Server configuration loaded",
		zap.String("ServerAddr", cfg.Server.Address),
		zap.String("BaseURL", cfg.Server.BaseURL),
	)

	if cfg.Profiling.EnablePprof {
		p := profiler.New()
		p.Start()
		defer p.Close()
		logger.Info("pprof server started", zap.String("address", p.Addr()))
	}

	var repo repository.URLRepositoryInterface

	if cfg.Storage.DatabaseDSN != "" {
		logger.Info("Using Database")
		dbRepo, err := repository.NewPostgresRepository(cfg.Storage.DatabaseDSN)
		if err != nil {
			logger.Fatal("Error connecting to DB", zap.Error(err))
		}
		defer dbRepo.Close()
		repo = dbRepo
	} else if cfg.Storage.FileStoragePath != "" {
		logger.Info("Using file storage",
			zap.String("file_path", cfg.Storage.FileStoragePath),
		)
		repo = repository.NewFileURLRepository(cfg.Storage.FileStoragePath)
	} else {
		logger.Info("Using in-memory storage")
		repo = repository.NewURLRepository()
	}

	svc := service.NewShortenerService(repo)

	auditSubject := audit.NewSubject()
	if cfg.Audit.FilePath != "" {
		fo, err := audit.NewFileObserver(cfg.Audit.FilePath)
		if err != nil {
			logger.Fatal("failed to create file audit observer", zap.Error(err))
		}
		auditSubject.Register(fo)
		logger.Info("Using file audit", zap.String("file_path", cfg.Audit.FilePath))
	}
	if cfg.Audit.URL != "" {
		auditSubject.Register(audit.NewHTTPObserver(cfg.Audit.URL))
		logger.Info("Using HTTP audit", zap.String("url", cfg.Audit.URL))
	}

	router := server.NewRouter(server.RouterDeps{
		BaseURL:      cfg.Server.BaseURL,
		CookieSecret: cfg.Security.CookieSecret,
		Logger:       logger,
		Repo:         repo,
		Svc:          svc,
		AuditSubject: auditSubject,
	})

	srv := &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	logger.Info("Server running at",
		zap.String("address", cfg.Server.BaseURL),
		zap.Bool("https", cfg.Server.EnableHTTPS),
	)

	if cfg.Server.EnableHTTPS {
		tlsConfig, err := tlsconfig.NewServerConfig(cfg.Server.Address)
		if err != nil {
			logger.Fatal("failed to initialize TLS config", zap.Error(err))
		}

		listener, err := tls.Listen("tcp", cfg.Server.Address, tlsConfig)
		if err != nil {
			logger.Fatal("failed to start TLS listener", zap.Error(err))
		}

		logger.Info("HTTPS server started with self-signed certificate")

		go func() {
			if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
				logger.Error("server error", zap.Error(err))
			}
		}()
	} else {
		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("server starting failed", zap.Error(err))
			}
		}()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	<-stop
	logger.Info("Shutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown failed", zap.Error(err))
	}
	cancel()

	flushDone := make(chan struct{})
	go func() {
		auditSubject.Flush()
		close(flushDone)
	}()
	select {
	case <-flushDone:
	case <-time.After(5 * time.Second):
		logger.Warn("audit flush timed out, proceeding")
	}

	svcDone := make(chan struct{})
	go func() {
		if err := svc.Close(); err != nil {
			logger.Error("Service close error", zap.Error(err))
		}
		close(svcDone)
	}()
	select {
	case <-svcDone:
	case <-time.After(5 * time.Second):
		logger.Warn("service close timed out, proceeding")
	}

	auditDone := make(chan struct{})
	go func() {
		auditSubject.Close()
		close(auditDone)
	}()
	select {
	case <-auditDone:
	case <-time.After(5 * time.Second):
		logger.Warn("audit close timed out, proceeding")
	}

	logger.Info("Server stopped")
}
