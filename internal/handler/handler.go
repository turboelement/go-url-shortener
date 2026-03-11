package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"go-url-shortener/internal/logger"
	"go-url-shortener/internal/repository"
	"go-url-shortener/internal/service"

	"go.uber.org/zap"
)

type JSONRequest struct {
	URL string `json:"url"`
}

type JSONResponse struct {
	Result string `json:"result"`
}

type BatchRequest []service.BatchItem

type BatchResponse []service.BatchResult

func PostHandler(svc *service.ShortenerService, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromContext(r.Context())

		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Error("Cannot read request body", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		originalURL := strings.TrimSpace(string(body))
		if originalURL == "" {
			log.Error("URL cannot be empty", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		shortID, err := svc.Shorten(originalURL)
		isURLAlreadyExists := errors.Is(err, repository.ErrURLAlreadyExists)

		if err != nil && !isURLAlreadyExists {
			log.Error("Failed to shorten", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		shortURL, err := url.JoinPath(baseURL, shortID)
		if err != nil {
			log.Error("Failed to build short URL", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		status := http.StatusCreated
		if isURLAlreadyExists {
			status = http.StatusConflict
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(status)
		w.Write([]byte(shortURL))
	}
}

func PostJSONHandler(svc *service.ShortenerService, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromContext(r.Context())

		var req JSONRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Error("Invalid JSON", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if req.URL == "" {
			log.Error("URL is required")
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		shortID, err := svc.Shorten(req.URL)
		isURLAlreadyExists := errors.Is(err, repository.ErrURLAlreadyExists)

		if err != nil && !isURLAlreadyExists {
			log.Error("Failed to shorten", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		shortURL, err := url.JoinPath(baseURL, shortID)
		if err != nil {
			log.Error("Failed to build short URL", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		status := http.StatusCreated
		if isURLAlreadyExists {
			status = http.StatusConflict
		}

		resp := JSONResponse{
			Result: shortURL,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)

		json.NewEncoder(w).Encode(resp)
	}
}

func BatchShortenHandler(svc *service.ShortenerService, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromContext(r.Context())

		var req BatchRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Error("Invalid JSON", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		results, err := svc.BatchShorten(r.Context(), req)
		if err != nil {
			log.Error("Failed to shorten batch", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		for i := range results {
			results[i].ShortURL, err = url.JoinPath(baseURL, results[i].ShortURL)
			if err != nil {
				log.Error("Failed to build short URL", zap.Error(err))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		json.NewEncoder(w).Encode(results)
	}
}

func GetHandler(svc *service.ShortenerService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromContext(r.Context())

		id := r.PathValue("id")
		if id == "" {
			log.Error("Short ID is required")
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		originalURL, found := svc.GetOriginalURL(id)
		if !found {
			log.Error("Short URL not found")
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		w.Header().Set("Location", originalURL)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}
}

func PingHandler(repo repository.URLRepositoryInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromContext(r.Context())

		if err := repo.Ping(r.Context()); err != nil {
			log.Error("Error connecting to DB", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
