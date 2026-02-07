package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go-url-shortener/internal/repository"
	"go-url-shortener/internal/service"
)

func newTestService() *service.ShortenerService {
	repo := repository.NewURLRepository()
	return service.NewShortenerService(repo)
}

func setupTestServer() (*httptest.Server, *service.ShortenerService) {
	svc := newTestService()

	r := chi.NewRouter()
	r.Post("/", PostHandler(svc, "http://localhost:8080/"))
	r.Get("/{id}", GetHandler(svc))

	//NewServeMux specific: ServeMux returns 405 Method Not Allowed for unknown routes
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Short ID is required", http.StatusBadRequest)
	})

	return httptest.NewServer(r), svc
}

func TestPostHandler(t *testing.T) {
	ts, _ := setupTestServer()
	defer ts.Close()

	client := resty.New()
	client.SetBaseURL(ts.URL)

	type want struct {
		code        int
		bodyContent string
		contentType string
	}

	tests := []struct {
		name string
		body string
		want want
	}{
		{
			name: "positive test",
			body: "https://practicum.yandex.ru",
			want: want{
				code:        http.StatusCreated,
				bodyContent: "http://localhost:8080/",
				contentType: "text/plain",
			},
		},
		{
			name: "empty URL",
			body: "   ",
			want: want{
				code: http.StatusBadRequest,
			},
		},
		{
			name: "empty body",
			body: "",
			want: want{
				code: http.StatusBadRequest,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := client.R().
				SetHeader("Content-Type", "text/plain")

			if tt.body != "" {
				req.SetBody(tt.body)
			}

			resp, err := req.Post("/")
			require.NoError(t, err, "request failed")

			assert.Equal(t, tt.want.code, resp.StatusCode(), "status code mismatch")

			if tt.want.contentType != "" {
				assert.Equal(t, tt.want.contentType, resp.Header().Get("Content-Type"), "content-type mismatch")
			}

			if tt.want.bodyContent != "" {
				assert.Contains(t, resp.String(), tt.want.bodyContent, "body content mismatch")
			}
		})
	}
}

func TestGetHandler(t *testing.T) {
	ts, svc := setupTestServer()
	defer ts.Close()

	client := resty.New()
	client.SetBaseURL(ts.URL)

	// disable redirect following for testing Location header
	client.SetRedirectPolicy(resty.RedirectPolicyFunc(func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}))

	type want struct {
		code     int
		location string
	}

	tests := []struct {
		name        string
		originalURL string
		want        want
	}{
		{
			name:        "positive redirect",
			originalURL: "https://practicum.yandex.ru",
			want: want{
				code:     http.StatusTemporaryRedirect,
				location: "https://practicum.yandex.ru",
			},
		},
		{
			name: "no id → bad request",
			want: want{
				code: http.StatusBadRequest,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/"
			if tt.originalURL != "" {
				shortID, err := svc.Shorten(tt.originalURL)
				require.NoError(t, err, "failed to shorten url")
				path = "/" + shortID
			}

			resp, err := client.R().Get(path)
			require.NoError(t, err, "GET request failed")

			assert.Equal(t, tt.want.code, resp.StatusCode(), "status code mismatch")

			if tt.want.location != "" {
				assert.Equal(t, tt.want.location, resp.Header().Get("Location"), "Location header mismatch")
			}
		})
	}
}
