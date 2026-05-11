// Package handler_test demonstrates API endpoint usage examples.
package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"

	"go-url-shortener/internal/auth"
	"go-url-shortener/internal/handler"
	"go-url-shortener/internal/repository"
	"go-url-shortener/internal/service"

	"github.com/go-chi/chi/v5"
)

// Example demonstrates all URL shortener endpoints:
// POST / (plain text), POST /api/shorten (JSON), GET /{id} (redirect),
// POST /api/shorten/batch (batch), GET /api/user/urls (user URLs),
// DELETE /api/user/urls (delete), GET /ping (health check).
func Example() {
	repo := repository.NewURLRepository()
	svc := service.NewShortenerService(repo)
	defer svc.Close()

	baseURL := "http://localhost:8080"
	const testUserID = "test-user"

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.SetUserIDInContext(r.Context(), testUserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	r.Post("/", handler.PostHandler(svc, baseURL))
	r.Post("/api/shorten", handler.PostJSONHandler(svc, baseURL))
	r.Post("/api/shorten/batch", handler.BatchShortenHandler(svc, baseURL))
	r.Get("/{id}", handler.GetHandler(svc))
	r.Get("/api/user/urls", handler.GetUserURLsHandler(svc, baseURL))
	r.Delete("/api/user/urls", handler.DeleteUserURLsHandler(svc))
	r.Get("/ping", handler.PingHandler(repo))

	ts := httptest.NewServer(r)
	defer ts.Close()

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// 1. Create short URL (plain text).
	body := bytes.NewReader([]byte("https://practicum.yandex.ru"))
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/", body)
	req.Header.Set("Content-Type", "text/plain")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	shortURLBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	shortURL := string(bytes.TrimSpace(shortURLBytes))
	fmt.Println("POST / =>", resp.StatusCode)

	// 2. Create short URL (JSON).
	jsonBody, _ := json.Marshal(map[string]string{"url": "https://golang.org"})
	resp, err = client.Post(ts.URL+"/api/shorten", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		log.Fatal(err)
	}
	var jsonResp struct {
		Result string `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&jsonResp)
	resp.Body.Close()
	fmt.Println("POST /api/shorten =>", resp.StatusCode)

	// 3. Follow short URL (redirect).
	shortID := shortURL[len(baseURL)+1:]
	resp, err = client.Get(ts.URL + "/" + shortID)
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
	fmt.Println("GET /{id} =>", resp.StatusCode, "Location:", resp.Header.Get("Location"))

	// 4. Batch shorten URLs.
	batch := []map[string]string{
		{"correlation_id": "a", "original_url": "https://ya.ru"},
		{"correlation_id": "b", "original_url": "https://google.com"},
	}
	batchBody, _ := json.Marshal(batch)
	resp, err = client.Post(ts.URL+"/api/shorten/batch", "application/json", bytes.NewReader(batchBody))
	if err != nil {
		log.Fatal(err)
	}
	var batchResp []struct {
		CorrelationID string `json:"correlation_id"`
		ShortURL      string `json:"short_url"`
	}
	json.NewDecoder(resp.Body).Decode(&batchResp)
	resp.Body.Close()
	fmt.Println("POST /api/shorten/batch =>", resp.StatusCode)

	// 5. Get user URLs.
	resp, err = client.Get(ts.URL + "/api/user/urls")
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
	fmt.Println("GET /api/user/urls =>", resp.StatusCode)

	// 6. Delete user URLs.
	delBody, _ := json.Marshal([]string{shortID})
	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/user/urls", bytes.NewReader(delBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
	fmt.Println("DELETE /api/user/urls =>", resp.StatusCode)

	// 7. Health check (ping).
	resp, err = client.Get(ts.URL + "/ping")
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
	fmt.Println("GET /ping =>", resp.StatusCode)

	// Output:
	// POST / => 201
	// POST /api/shorten => 201
	// GET /{id} => 307 Location: https://practicum.yandex.ru
	// POST /api/shorten/batch => 201
	// GET /api/user/urls => 200
	// DELETE /api/user/urls => 202
	// GET /ping => 200
}
