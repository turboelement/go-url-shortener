package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"go-url-shortener/internal/repository"
	"go-url-shortener/internal/service"
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
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Cannot read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		originalURL := strings.TrimSpace(string(body))
		if originalURL == "" {
			http.Error(w, "URL cannot be empty", http.StatusBadRequest)
			return
		}

		shortID, err := svc.Shorten(originalURL)
		if err != nil {
			http.Error(w, "Failed to generate short URL", http.StatusInternalServerError)
			return
		}

		shortURL := baseURL + "/" + shortID

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(shortURL))
	}
}

func PostJSONHandler(svc *service.ShortenerService, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req JSONRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if req.URL == "" {
			http.Error(w, "URL is required", http.StatusBadRequest)
			return
		}

		shortID, err := svc.Shorten(req.URL)
		if err != nil {
			http.Error(w, "Failed to shorten", http.StatusInternalServerError)
			return
		}

		shortURL := baseURL + "/" + shortID

		resp := JSONResponse{
			Result: shortURL,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		// enc := json.NewEncoder(w)
		// enc.SetIndent("", "  ")
		// enc.Encode(resp)

		json.NewEncoder(w).Encode(resp)
	}
}

func BatchShortenHandler(svc *service.ShortenerService, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req BatchRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		results, err := svc.BatchShorten(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		for i := range results {
			results[i].ShortURL = baseURL + "/" + results[i].ShortURL
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		json.NewEncoder(w).Encode(results)
	}
}

func GetHandler(svc *service.ShortenerService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "Short ID is required", http.StatusBadRequest)
			return
		}

		originalURL, found := svc.GetOriginalURL(id)
		if !found {
			http.Error(w, "Short URL not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Location", originalURL)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}
}

func PingHandler(repo repository.URLRepositoryInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := repo.Ping(r.Context()); err != nil {
			http.Error(w, "Error connecting to DB", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
