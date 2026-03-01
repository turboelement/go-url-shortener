package handler

import (
	"encoding/json"
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

const testBaseURL = "http://localhost:8080"

func newTestService() *service.ShortenerService {
	repo := repository.NewURLRepository()
	return service.NewShortenerService(repo)
}

func setupTestServer() (*httptest.Server, *service.ShortenerService) {
	svc := newTestService()

	r := chi.NewRouter()
	r.Post("/", PostHandler(svc, testBaseURL))
	r.Post("/api/shorten", PostJSONHandler(svc, testBaseURL))
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

func TestPostJSONHandler(t *testing.T) {
	ts, _ := setupTestServer()
	defer ts.Close()

	client := resty.New()
	client.SetBaseURL(ts.URL)

	tests := []struct {
		name            string
		requestBody     interface{} // map, struct or []byte
		wantStatus      int
		wantContain     string
		wantContentType string
	}{
		{
			name: "positive test — valid URL",
			requestBody: map[string]string{
				"url": "https://practicum.yandex.ru",
			},
			wantStatus:      http.StatusCreated,
			wantContain:     testBaseURL + "/",
			wantContentType: "application/json",
		},
		{
			name: "empty URL in JSON",
			requestBody: map[string]string{
				"url": "",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing url field",
			requestBody: map[string]string{
				"link": "https://ya.ru",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:        "invalid JSON",
			requestBody: `{"url": "https://ya.ru",}`, // extra comma
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "not JSON at all",
			requestBody: "just plain text",
			wantStatus:  http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			var err error

			switch v := tt.requestBody.(type) {
			case string:
				body = []byte(v)
			case map[string]string:
				body, err = json.Marshal(v)
				require.NoError(t, err)
			default:
				t.Fatalf("unsupported requestBody type: %T", v)
			}

			resp, err := client.R().
				SetHeader("Content-Type", "application/json").
				SetBody(body).
				Post("/api/shorten")

			require.NoError(t, err)

			assert.Equal(t, tt.wantStatus, resp.StatusCode(), "status code mismatch")

			if tt.wantContentType != "" {
				assert.Equal(t, tt.wantContentType, resp.Header().Get("Content-Type"))
			}

			if tt.wantContain != "" {
				assert.Contains(t, resp.String(), tt.wantContain, "response should contain short URL prefix")
				assert.Contains(t, resp.String(), `"result":"`, "should have result field")
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
