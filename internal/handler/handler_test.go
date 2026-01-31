package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-url-shortener/internal/repository"
	"go-url-shortener/internal/service"
)

func newTestService() *service.ShortenerService {
	repo := repository.NewURLRepository()
	return service.NewShortenerService(repo)
}

func setupTestMux(svc *service.ShortenerService, baseURL string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /", PostHandler(svc, baseURL))
	mux.HandleFunc("GET /{id}", GetHandler(svc))

	//NewServeMux specific: ServeMux returns 405 Method Not Allowed for unknown routes
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Short ID is required", http.StatusBadRequest)
	})
	return mux
}

func TestPostHandler(t *testing.T) {
	type want struct {
		code        int
		bodyContent string
		contentType string
	}

	tests := []struct {
		name        string
		requestBody string
		want        want
	}{
		{
			name:        "positive test",
			requestBody: "https://practicum.yandex.ru",
			want: want{
				code:        http.StatusCreated,
				bodyContent: "http://localhost:8080/",
				contentType: "text/plain",
			},
		},
		{
			name:        "empty URL",
			requestBody: "   ",
			want: want{
				code: http.StatusBadRequest,
			},
		},
		{
			name:        "empty body",
			requestBody: "",
			want: want{
				code: http.StatusBadRequest,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService()
			mux := setupTestMux(svc, "http://localhost:8080")

			var body io.Reader
			if tt.requestBody != "" {
				body = strings.NewReader(tt.requestBody)
			}

			req := httptest.NewRequest(http.MethodPost, "/", body)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != tt.want.code {
				t.Errorf("expected status %d, got %d", tt.want.code, res.StatusCode)
				return
			}

			if tt.want.contentType != "" {
				gotType := res.Header.Get("Content-Type")
				if gotType != tt.want.contentType {
					t.Errorf("expected Content-Type %q, got %q", tt.want.contentType, gotType)
				}
			}

			if tt.want.bodyContent != "" {
				bodyBytes, err := io.ReadAll(res.Body)
				if err != nil {
					t.Fatalf("failed to read response body: %v", err)
				}
				bodyStr := string(bodyBytes)
				if !strings.Contains(bodyStr, tt.want.bodyContent) {
					t.Errorf("expected body to contain %q, got %q", tt.want.bodyContent, bodyStr)
				}
			}
		})
	}
}

func TestGetHandler(t *testing.T) {
	type want struct {
		code     int
		location string
	}

	tests := []struct {
		name string
		path string
		want want
	}{
		{
			name: "positive test",
			path: "https://practicum.yandex.ru",
			want: want{
				code:     http.StatusTemporaryRedirect,
				location: "https://practicum.yandex.ru",
			},
		},
		{
			name: "empty path",
			want: want{
				code: http.StatusBadRequest,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService()
			mux := setupTestMux(svc, "http://localhost:8080")

			var path string
			if tt.path != "" {
				shortID, err := svc.Shorten(tt.path)
				if err != nil {
					t.Fatalf("failed to shorten url: %v", err)
				}
				path = "/" + shortID
			} else {
				path = "/"
			}

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != tt.want.code {
				t.Errorf("expected code %d, got %d", tt.want.code, res.StatusCode)
			}

			if tt.want.location != "" {
				gotLocation := res.Header.Get("Location")
				if gotLocation != tt.want.location {
					t.Errorf("expected Location %q, got %q", tt.want.location, gotLocation)
				}
			}
		})
	}
}
