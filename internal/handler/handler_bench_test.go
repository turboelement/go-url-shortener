package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"go-url-shortener/internal/auth"
	"go-url-shortener/internal/repository"
	"go-url-shortener/internal/service"

	"github.com/go-chi/chi/v5"
)

// setupBenchServer creates a test server with in-memory repository for benchmarks
func setupBenchServer(b *testing.B) (*httptest.Server, *service.ShortenerService) {
	b.Helper()

	repo := repository.NewURLRepository()
	svc := service.NewShortenerService(repo)

	r := chi.NewRouter()

	// Add middleware, set userID in context
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.SetUserIDInContext(r.Context(), benchUserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	r.Post("/", PostHandler(svc, benchBaseURL))
	r.Post("/api/shorten", PostJSONHandler(svc, benchBaseURL))
	r.Post("/api/shorten/batch", BatchShortenHandler(svc, benchBaseURL))
	r.Get("/{id}", GetHandler(svc))
	r.Get("/api/user/urls", GetUserURLsHandler(svc, benchBaseURL))
	r.Delete("/api/user/urls", DeleteUserURLsHandler(svc))

	return httptest.NewServer(r), svc
}

const (
	benchBaseURL = "http://localhost:8080"
	benchUserID  = "bench-user-id"
)

func BenchmarkPostHandler(b *testing.B) {
	ts, svc := setupBenchServer(b)
	defer ts.Close()
	defer svc.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		body := strings.NewReader("https://yandex.ru/search/?text=benchmark+test+query+" + strconv.Itoa(i))
		// Create new request each time since the body is consumed
		req := httptest.NewRequest(http.MethodPost, ts.URL+"/", body)
		w := httptest.NewRecorder()

		// Use handler directly
		ts.Config.Handler.ServeHTTP(w, req)

		if w.Code != http.StatusCreated && w.Code != http.StatusConflict {
			b.Fatalf("expected 201 or 409, got %d", w.Code)
		}
	}
}

func BenchmarkPostJSONHandler(b *testing.B) {
	ts, svc := setupBenchServer(b)
	defer ts.Close()
	defer svc.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		jsonBody := `{"url":"https://ya.ru/benchmark/path/` + strconv.Itoa(i) + `"}`
		req := httptest.NewRequest(
			http.MethodPost,
			ts.URL+"/api/shorten",
			strings.NewReader(jsonBody),
		)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		ts.Config.Handler.ServeHTTP(w, req)

		if w.Code != http.StatusCreated && w.Code != http.StatusConflict {
			b.Fatalf("expected 201 or 409, got %d", w.Code)
		}
	}
}

func BenchmarkBatchShortenHandler(b *testing.B) {
	ts, svc := setupBenchServer(b)
	defer ts.Close()
	defer svc.Close()

	// Pre-create different batch requests to avoid measuring JSON generation
	numBatches := 100
	batches := make([]string, numBatches)
	for i := 0; i < numBatches; i++ {
		batches[i] = `[{"correlation_id":"cid-` + strconv.Itoa(i) + `-1","original_url":"https://example.com/` + strconv.Itoa(i) + `/path1"},{"correlation_id":"cid-` + strconv.Itoa(i) + `-2","original_url":"https://example.com/` + strconv.Itoa(i) + `/path2"},{"correlation_id":"cid-` + strconv.Itoa(i) + `-3","original_url":"https://example.com/` + strconv.Itoa(i) + `/path3"}]`
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(
			http.MethodPost,
			ts.URL+"/api/shorten/batch",
			strings.NewReader(batches[i%numBatches]),
		)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		ts.Config.Handler.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			b.Fatalf("expected 201, got %d, body: %s", w.Code, w.Body.String())
		}
	}
}

func BenchmarkGetHandler(b *testing.B) {
	ts, svc := setupBenchServer(b)
	defer ts.Close()
	defer svc.Close()

	// Pre-save URL
	ctx := context.Background()
	shortID, err := svc.Shorten(ctx, "https://example.com/benchmark-get")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, ts.URL+"/"+shortID, nil)
		w := httptest.NewRecorder()
		ts.Config.Handler.ServeHTTP(w, req)

		if w.Code != http.StatusTemporaryRedirect {
			b.Fatalf("expected 307, got %d", w.Code)
		}
	}
}

func BenchmarkGetUserURLsHandler(b *testing.B) {
	ts, svc := setupBenchServer(b)
	defer ts.Close()
	defer svc.Close()

	// Pre-save URLs for user
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		_, err := svc.ShortenWithUser(ctx, "https://example.com/user/"+strconv.Itoa(i), benchUserID)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, ts.URL+"/api/user/urls", nil)
		w := httptest.NewRecorder()
		ts.Config.Handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("expected 200, got %d", w.Code)
		}
	}
}
