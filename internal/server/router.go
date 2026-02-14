package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"go-url-shortener/internal/handler"
	"go-url-shortener/internal/logger"
	"go-url-shortener/internal/repository"
	"go-url-shortener/internal/service"
)

func NewRouter(baseURL string) http.Handler {
	repo := repository.NewURLRepository()
	svc := service.NewShortenerService(repo)

	r := chi.NewRouter()

	r.Post("/", handler.PostHandler(svc, baseURL))
	r.Post("/api/shorten", handler.PostJSONHandler(svc, baseURL))
	r.Get("/{id}", handler.GetHandler(svc))

	return r
}

func NewRouterWithLogger(baseURL string, zaplogger *zap.Logger) http.Handler {
	repo := repository.NewURLRepository()
	svc := service.NewShortenerService(repo)

	r := chi.NewRouter()

	r.Use(logger.Logger(zaplogger))

	r.Post("/", handler.PostHandler(svc, baseURL))
	r.Post("/api/shorten", handler.PostJSONHandler(svc, baseURL))
	r.Get("/{id}", handler.GetHandler(svc))

	return r
}
