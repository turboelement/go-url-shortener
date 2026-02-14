package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"go-url-shortener/internal/config"
	"go-url-shortener/internal/server"
)

func main() {
	cfg := config.New()

	router := server.NewRouter(cfg.BaseURL)

	srv := &http.Server{
		Addr:         cfg.ServerAddr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	fmt.Printf("Server running at %s\n", cfg.BaseURL)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
