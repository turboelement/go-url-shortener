package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"go-url-shortener/internal/config"
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

	router := server.NewRouterWithLogger(cfg.BaseURL, logger, cfg.FileStoragePath)
	//router := server.NewRouter(cfg.BaseURL)

	srv := &http.Server{
		Addr:         cfg.ServerAddr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	fmt.Printf("Server running at %s\n", cfg.BaseURL)

	if err := srv.ListenAndServe(); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}
