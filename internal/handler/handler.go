// Package handler provides HTTP handlers for URL shortener endpoints.
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go-url-shortener/internal/audit"
	"go-url-shortener/internal/logger"
	"go-url-shortener/internal/middleware"
	"go-url-shortener/internal/repository"
	"go-url-shortener/internal/service"

	"go.uber.org/zap"
)

// matches the original_url VARCHAR(4096) field limit in the db
const maxOriginalURLLength = 4096

// JSONRequest is the request body for the JSON shorten endpoint.
type JSONRequest struct {
	URL string `json:"url"`
}

// JSONResponse is the response from the JSON shorten endpoint.
type JSONResponse struct {
	Result string `json:"result"`
}

// BatchRequest is a list of URLs to shorten in one batch.
type BatchRequest []service.BatchItem

// BatchResponse contains the shortened URLs for a batch request.
type BatchResponse []service.BatchResult

// UserURLsResponse lists all URLs belonging to a user.
type UserURLsResponse []UserURLItem

// UserURLItem is a single short-original URL pair for a user.
type UserURLItem struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// StatsResponse contains service statistics (URLs and users counts).
type StatsResponse struct {
	URLs  int `json:"urls"`
	Users int `json:"users"`
}

// PostHandler handles POST /. Reads a plain-text URL and returns a short URL.
func PostHandler(svc *service.ShortenerService, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromContext(r.Context())

		r.Body = http.MaxBytesReader(w, r.Body, maxOriginalURLLength+1024)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Debug("Cannot read request body", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		originalURL := strings.TrimSpace(string(body))
		if originalURL == "" {
			log.Debug("URL cannot be empty", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if len(originalURL) > maxOriginalURLLength {
			log.Debug("URL exceeds maximum allowed length",
				zap.Int("length", len(originalURL)),
				zap.Int("max", maxOriginalURLLength),
			)
			http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
			return
		}

		userID, err := middleware.GetUserIDFromContext(r)
		if err != nil {
			log.Debug("Failed to get User ID from context", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		shortID, err := svc.ShortenWithUser(r.Context(), originalURL, userID)
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

		if status == http.StatusCreated {
			as := audit.FromContext(r.Context())
			if as != nil {
				as.NotifyAll(audit.AuditEvent{
					Timestamp: time.Now().Unix(),
					Action:    audit.ActionShorten,
					UserID:    userID,
					URL:       originalURL,
				})
			}
		}
	}
}

// PostJSONHandler handles POST /api/shorten. Reads JSON with a URL and returns a short URL as JSON.
func PostJSONHandler(svc *service.ShortenerService, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromContext(r.Context())

		var req JSONRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Debug("Invalid JSON", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if req.URL == "" {
			log.Debug("URL is required")
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if len(req.URL) > maxOriginalURLLength {
			log.Debug("URL exceeds maximum allowed length",
				zap.Int("length", len(req.URL)),
				zap.Int("max", maxOriginalURLLength),
			)
			http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
			return
		}

		userID, err := middleware.GetUserIDFromContext(r)
		if err != nil {
			log.Debug("Failed to get User ID from context", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		shortID, err := svc.ShortenWithUser(r.Context(), req.URL, userID)
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

		if status == http.StatusCreated {
			as := audit.FromContext(r.Context())
			if as != nil {
				as.NotifyAll(audit.AuditEvent{
					Timestamp: time.Now().Unix(),
					Action:    audit.ActionShorten,
					UserID:    userID,
					URL:       req.URL,
				})
			}
		}
	}
}

// BatchShortenHandler handles POST /api/shorten/batch. Shortens multiple URLs at once.
func BatchShortenHandler(svc *service.ShortenerService, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromContext(r.Context())

		var req BatchRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Debug("Invalid JSON", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		userID, err := middleware.GetUserIDFromContext(r)
		if err != nil {
			log.Debug("Failed to get User ID from context", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		results, err := svc.BatchShortenWithUser(r.Context(), userID, req)
		if err != nil {
			log.Debug("Failed to shorten batch", zap.Error(err))
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

// GetHandler handles GET /{id}. Redirects to the original URL via HTTP 307.
func GetHandler(svc *service.ShortenerService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromContext(r.Context())

		id := r.PathValue("id")
		if id == "" {
			log.Debug("Short ID is required")
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		originalURL, err := svc.GetOriginalURL(r.Context(), id)
		if err != nil {
			if errors.Is(err, repository.ErrURLMarkedAsDeleted) {
				log.Debug("Short URL marked as deleted")
				http.Error(w, http.StatusText(http.StatusGone), http.StatusGone)
				return
			}
			if errors.Is(err, repository.ErrURLNotFound) {
				log.Debug("Short URL not found")
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}
			log.Error("Error getting URL", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if originalURL == "" {
			log.Debug("Short URL not found")
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		w.Header().Set("Location", originalURL)
		w.WriteHeader(http.StatusTemporaryRedirect)

		as := audit.FromContext(r.Context())
		if as != nil {
			userID, err := middleware.GetUserIDFromContext(r)
			if err != nil {
				userID = ""
			}

			as.NotifyAll(audit.AuditEvent{
				Timestamp: time.Now().Unix(),
				Action:    audit.ActionFollow,
				UserID:    userID,
				URL:       originalURL,
			})
		}
	}
}

// GetUserURLsHandler handles GET /api/user/urls. Returns all URLs created by the current user.
func GetUserURLsHandler(svc *service.ShortenerService, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromContext(r.Context())

		userID, err := middleware.GetUserIDFromContext(r)
		if err != nil {
			log.Debug("Failed to get User ID from context", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		userURLs, err := svc.GetUserURLs(r.Context(), userID)
		if err != nil {
			log.Error("Failed to get user URLs", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if len(userURLs) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		response := make(UserURLsResponse, 0, len(userURLs))
		for _, u := range userURLs {
			shortURL, err := url.JoinPath(baseURL, u.ShortURL)
			if err != nil {
				log.Error("Failed to build short URL", zap.Error(err))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			response = append(response, UserURLItem{
				ShortURL:    shortURL,
				OriginalURL: u.OriginalURL,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

// DeleteUserURLsHandler handles DELETE /api/user/urls. Marks the user's URLs as deleted (async).
func DeleteUserURLsHandler(svc *service.ShortenerService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromContext(r.Context())

		userID, err := middleware.GetUserIDFromContext(r)
		if err != nil {
			log.Debug("Failed to get User ID from context", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		var shortIDs []string
		if err := json.NewDecoder(r.Body).Decode(&shortIDs); err != nil {
			log.Debug("Invalid JSON", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if len(shortIDs) == 0 {
			log.Debug("Empty short IDs list")
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		svc.DeleteUserURLsAsync(userID, shortIDs)

		w.WriteHeader(http.StatusAccepted)
	}
}

// StatsHandler handles GET /api/internal/stats.
// Returns service statistics if the client's IP is in the trusted subnet.
func StatsHandler(svc *service.ShortenerService, trustedSubnet string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromContext(r.Context())

		if !middleware.CheckTrustedSubnet(r, trustedSubnet) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		urlsCount, usersCount, err := svc.GetStats(r.Context())
		if err != nil {
			log.Error("Failed to get stats", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		resp := StatsResponse{
			URLs:  urlsCount,
			Users: usersCount,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

// PingHandler handles GET /ping. Checks database connectivity, returns 200 if OK.
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
