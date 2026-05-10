package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware" //Compress
	"go.uber.org/zap"

	"go-url-shortener/internal/audit"
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
	Svc          *service.ShortenerService
	AuditSubject *audit.Subject
}

func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Decompress)
	r.Use(logger.Logger(deps.Logger))
	// compression level of 5 is sensible value
	r.Use(chimiddleware.Compress(5, "application/json", "text/html"))

	r.Use(middleware.AuthMiddleware(deps.CookieSecret, deps.Logger))

	r.Use(audit.Middleware(deps.AuditSubject))

	r.Post("/", handler.PostHandler(deps.Svc, deps.BaseURL))
	r.Post("/api/shorten", handler.PostJSONHandler(deps.Svc, deps.BaseURL))
	r.Post("/api/shorten/batch", handler.BatchShortenHandler(deps.Svc, deps.BaseURL))
	r.Get("/{id}", handler.GetHandler(deps.Svc))
	r.Get("/ping", handler.PingHandler(deps.Repo))
	r.Get("/api/user/urls", handler.GetUserURLsHandler(deps.Svc, deps.BaseURL))
	r.Delete("/api/user/urls", handler.DeleteUserURLsHandler(deps.Svc))

	return r
}
