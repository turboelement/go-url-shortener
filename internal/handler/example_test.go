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

// setupTestRouter creates a test server with all routes for the examples.
func setupTestRouter() (*httptest.Server, *http.Client, string) {
	repo := repository.NewURLRepository()
	svc := service.NewShortenerService(repo)

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

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return ts, client, baseURL
}

// ExamplePostHandler demonstrates creating a short URL via plain text POST.
func ExamplePostHandler() {
	ts, client, _ := setupTestRouter()
	defer ts.Close()

	body := bytes.NewReader([]byte("https://practicum.yandex.ru"))
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/", body)
	req.Header.Set("Content-Type", "text/plain")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
	fmt.Println("POST / =>", resp.StatusCode)

	// Output:
	// POST / => 201
}

// ExamplePostJSONHandler demonstrates creating a short URL via JSON POST.
func ExamplePostJSONHandler() {
	ts, client, _ := setupTestRouter()
	defer ts.Close()

	jsonBody, _ := json.Marshal(map[string]string{"url": "https://golang.org"})
	resp, err := client.Post(ts.URL+"/api/shorten", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		log.Fatal(err)
	}
	var jsonResp struct {
		Result string `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&jsonResp)
	resp.Body.Close()
	fmt.Println("POST /api/shorten =>", resp.StatusCode)

	// Output:
	// POST /api/shorten => 201
}

// ExampleGetHandler demonstrates following a short URL redirect.
func ExampleGetHandler() {
	ts, client, baseURL := setupTestRouter()
	defer ts.Close()

	// First, create a short URL.
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

	// Follow the short URL.
	shortID := shortURL[len(baseURL)+1:]
	resp, err = client.Get(ts.URL + "/" + shortID)
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
	fmt.Println("GET /{id} =>", resp.StatusCode, "Location:", resp.Header.Get("Location"))

	// Output:
	// GET /{id} => 307 Location: https://practicum.yandex.ru
}

// ExampleBatchShortenHandler demonstrates batch URL shortening.
func ExampleBatchShortenHandler() {
	ts, client, _ := setupTestRouter()
	defer ts.Close()

	batch := []map[string]string{
		{"correlation_id": "a", "original_url": "https://ya.ru"},
		{"correlation_id": "b", "original_url": "https://google.com"},
	}
	batchBody, _ := json.Marshal(batch)
	resp, err := client.Post(ts.URL+"/api/shorten/batch", "application/json", bytes.NewReader(batchBody))
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

	// Output:
	// POST /api/shorten/batch => 201
}

// ExampleGetUserURLsHandler demonstrates retrieving all user URLs.
func ExampleGetUserURLsHandler() {
	ts, client, _ := setupTestRouter()
	defer ts.Close()

	resp, err := client.Get(ts.URL + "/api/user/urls")
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
	fmt.Println("GET /api/user/urls =>", resp.StatusCode)

	// Output:
	// GET /api/user/urls => 204
}

// ExampleDeleteUserURLsHandler demonstrates deleting user URLs.
func ExampleDeleteUserURLsHandler() {
	ts, client, baseURL := setupTestRouter()
	defer ts.Close()

	// First, create a short URL to delete.
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
	shortID := shortURL[len(baseURL)+1:]

	// Delete the short URL.
	delBody, _ := json.Marshal([]string{shortID})
	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/user/urls", bytes.NewReader(delBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
	fmt.Println("DELETE /api/user/urls =>", resp.StatusCode)

	// Output:
	// DELETE /api/user/urls => 202
}

// ExamplePingHandler demonstrates the health check endpoint.
func ExamplePingHandler() {
	ts, client, _ := setupTestRouter()
	defer ts.Close()

	resp, err := client.Get(ts.URL + "/ping")
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
	fmt.Println("GET /ping =>", resp.StatusCode)

	// Output:
	// GET /ping => 200
}
