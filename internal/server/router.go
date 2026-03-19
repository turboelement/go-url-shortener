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

type RouterDeps struct {
	BaseURL      string
	CookieSecret string
	Logger       *zap.Logger
	Repo         repository.URLRepositoryInterface
}

func NewRouter(deps RouterDeps) http.Handler {
	svc := service.NewShortenerService(deps.Repo)

	r := chi.NewRouter()
	r.Use(middleware.Decompress)
	r.Use(logger.Logger(deps.Logger))
	// compression level of 5 is sensible value
	r.Use(chimiddleware.Compress(5, "application/json", "text/html"))

	r.Use(middleware.AuthMiddleware(deps.CookieSecret, deps.Logger))

	r.Post("/", handler.PostHandler(svc, deps.BaseURL))
	r.Post("/api/shorten", handler.PostJSONHandler(svc, deps.BaseURL))
	r.Post("/api/shorten/batch", handler.BatchShortenHandler(svc, deps.BaseURL))
	r.Get("/{id}", handler.GetHandler(svc))
	r.Get("/ping", handler.PingHandler(deps.Repo))
	r.Get("/api/user/urls", handler.GetUserURLsHandler(svc, deps.BaseURL))

	return r
}
