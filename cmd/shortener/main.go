package main

import (
	"fmt"
	"log"
	"net/http"

	"go-url-shortener/internal/handler"
	"go-url-shortener/internal/repository"
	"go-url-shortener/internal/service"
)

func main() {
	const (
		addr    = ":8080"
		baseURL = "http://localhost:8080"
	)

	repo := repository.NewURLRepository()
	svc := service.NewShortenerService(repo)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /", handler.PostHandler(svc, baseURL))
	mux.HandleFunc("GET /{id}", handler.GetHandler(svc))

	fmt.Printf("Server running at %s\n", baseURL)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
