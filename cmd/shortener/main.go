package main

import (
	"fmt"
	"log"
	"net/http"

	"go-url-shortener/internal/config"
	"go-url-shortener/internal/server"
)

func main() {
	cfg := config.New()

	router := server.NewRouter(cfg.BaseURL)

	fmt.Printf("Server running at %s\n", cfg.BaseURL)

	if err := http.ListenAndServe(cfg.ServerAddr, router); err != nil {
		log.Fatal(err)
	}
}
