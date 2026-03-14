package main

import (
	"log"
	"net/http"
	"time"

	"go-url-shortener/internal/config"
	"go-url-shortener/internal/repository"
	"go-url-shortener/internal/server"

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

	router := server.NewRouter(server.RouterDeps{
		BaseURL: cfg.BaseURL,
		Logger:  logger,
		Repo:    repo,
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

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server failed", zap.Error(err))
	}
}
