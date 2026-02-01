package main

import (
	"fmt"
	"log"
	"net/http"

	"go-url-shortener/internal/server"
)

const (
	addr    = ":8080"
	baseURL = "http://localhost:8080"
)

func main() {
	router := server.NewRouter(baseURL)

	fmt.Printf("Server running at %s\n", baseURL)

	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}
