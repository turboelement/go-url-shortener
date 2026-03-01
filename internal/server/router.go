package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware" //Compress
	"go.uber.org/zap"

	"go-url-shortener/internal/handler"
	"go-url-shortener/internal/logger"
	"go-url-shortener/internal/middleware"
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

func NewRouterWithLogger(baseURL string, zaplogger *zap.Logger, filePath string) http.Handler {
	// repo := repository.NewURLRepository()
	repo := repository.NewFileURLRepository(filePath)
	svc := service.NewShortenerService(repo)

	r := chi.NewRouter()

	r.Use(middleware.Decompress)

	r.Use(logger.Logger(zaplogger))

	// compression level of 5 is sensible value
	r.Use(chimiddleware.Compress(5, "application/json", "text/html"))

	r.Post("/", handler.PostHandler(svc, baseURL))
	r.Post("/api/shorten", handler.PostJSONHandler(svc, baseURL))
	r.Get("/{id}", handler.GetHandler(svc))

	return r
}
