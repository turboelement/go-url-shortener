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
	BaseURL  string
	Logger   *zap.Logger
	FilePath string
	DBRepo   *repository.PostgresRepository
}

func NewRouter(deps RouterDeps) http.Handler {
	var repo repository.URLRepositoryInterface

	if deps.DBRepo != nil {
		repo = deps.DBRepo
	} else if deps.FilePath != "" {
		repo = repository.NewFileURLRepository(deps.FilePath)
	} else {
		repo = repository.NewURLRepository()
	}

	svc := service.NewShortenerService(repo)

	r := chi.NewRouter()
	r.Use(middleware.Decompress)
	r.Use(logger.Logger(deps.Logger))
	// compression level of 5 is sensible value
	r.Use(chimiddleware.Compress(5, "application/json", "text/html"))

	r.Post("/", handler.PostHandler(svc, deps.BaseURL))
	r.Post("/api/shorten", handler.PostJSONHandler(svc, deps.BaseURL))
	r.Get("/{id}", handler.GetHandler(svc))
	r.Get("/ping", handler.PingHandler(repo))

	return r
}
