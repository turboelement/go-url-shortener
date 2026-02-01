package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"go-url-shortener/internal/handler"
	"go-url-shortener/internal/repository"
	"go-url-shortener/internal/service"
)

func NewRouter(baseURL string) http.Handler {
	repo := repository.NewURLRepository()
	svc := service.NewShortenerService(repo)

	r := chi.NewRouter()

	r.Post("/", handler.PostHandler(svc, baseURL))
	r.Get("/{id}", handler.GetHandler(svc))

	return r
}
