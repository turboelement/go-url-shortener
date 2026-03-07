package main

import (
	"fmt"
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

	var dbRepo *repository.PostgresRepository
	if cfg.DatabaseDSN != "" {
		var err error
		dbRepo, err = repository.NewPostgresRepository(cfg.DatabaseDSN)
		if err != nil {
			logger.Fatal("Error connecting to DB", zap.Error(err))
		}
		defer dbRepo.Close()
	}

	router := server.NewRouter(server.RouterDeps{
		BaseURL:  cfg.BaseURL,
		Logger:   logger,
		FilePath: cfg.FileStoragePath,
		DBRepo:   dbRepo,
	})

	srv := &http.Server{
		Addr:         cfg.ServerAddr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	fmt.Printf("Server running at %s\n", cfg.BaseURL)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server failed", zap.Error(err))
	}
}
