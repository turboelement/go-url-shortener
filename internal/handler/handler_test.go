package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"go-url-shortener/internal/repository/mocks"
	"go-url-shortener/internal/service"
)

const testBaseURL = "http://localhost:8080"

func setupTestServer(t *testing.T) (*httptest.Server, *mocks.MockURLRepositoryInterface) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockURLRepositoryInterface(ctrl)

	svc := service.NewShortenerService(mockRepo)

	r := chi.NewRouter()
	r.Post("/", PostHandler(svc, testBaseURL))
	r.Post("/api/shorten", PostJSONHandler(svc, testBaseURL))
	r.Post("/api/shorten/batch", BatchShortenHandler(svc, testBaseURL))
	r.Get("/{id}", GetHandler(svc))
	r.Get("/ping", PingHandler(mockRepo))

	//NewServeMux specific: ServeMux returns 405 Method Not Allowed for unknown routes
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Short ID is required", http.StatusBadRequest)
	})

	return httptest.NewServer(r), mockRepo
}

func TestPostHandler(t *testing.T) {
	ts, mockRepo := setupTestServer(t)
	defer ts.Close()

	client := resty.New()
	client.SetBaseURL(ts.URL)

	type want struct {
		code int
		body string
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
				code: http.StatusCreated,
				body: testBaseURL + "/",
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
			trimmed := strings.TrimSpace(tt.body)
			if trimmed != "" {
				mockRepo.EXPECT().Get(gomock.Any()).Return("", false).AnyTimes()

				mockRepo.EXPECT().Save(gomock.Any(), trimmed).Times(1)
			}

			req := client.R().
				SetHeader("Content-Type", "text/plain")

			if tt.body != "" {
				req.SetBody(tt.body)
			}

			resp, err := req.Post("/")
			require.NoError(t, err, "request failed")

			assert.Equal(t, tt.want.code, resp.StatusCode(), "status code mismatch")

			if tt.want.body != "" {
				assert.Contains(t, resp.String(), tt.want.body, "body content mismatch")
			}
		})
	}
}

func TestPostJSONHandler(t *testing.T) {
	ts, mockRepo := setupTestServer(t)
	defer ts.Close()

	client := resty.New()
	client.SetBaseURL(ts.URL)

	type want struct {
		code int
		body string
	}

	tests := []struct {
		name string
		body interface{} // map, struct or []byte
		want want
	}{
		{
			name: "positive test — valid URL",
			body: map[string]string{
				"url": "https://practicum.yandex.ru",
			},
			want: want{
				code: http.StatusCreated,
				body: testBaseURL + "/",
			},
		},
		{
			name: "empty URL in JSON",
			body: map[string]string{
				"url": "",
			},
			want: want{
				code: http.StatusBadRequest,
			},
		},
		{
			name: "missing url field",
			body: map[string]string{
				"link": "https://ya.ru",
			},
			want: want{
				code: http.StatusBadRequest,
			},
		},
		{
			name: "invalid JSON",
			body: `{"url": "https://ya.ru",}`, // extra comma
			want: want{
				code: http.StatusBadRequest,
			},
		},
		{
			name: "not JSON at all",
			body: "just plain text",
			want: want{
				code: http.StatusBadRequest,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			var err error

			switch v := tt.body.(type) {
			case string:
				body = []byte(v)
			case map[string]string:
				body, err = json.Marshal(v)
				require.NoError(t, err)
			default:
				t.Fatalf("unsupported body type: %T", v)
			}

			if tt.want.code == http.StatusCreated {
				mockRepo.EXPECT().Get(gomock.Any()).Return("", false).AnyTimes()
				mockRepo.EXPECT().Save(gomock.Any(), tt.body.(map[string]string)["url"]).Times(1)
			}

			resp, err := client.R().
				SetHeader("Content-Type", "application/json").
				SetBody(body).
				Post("/api/shorten")

			require.NoError(t, err)

			assert.Equal(t, tt.want.code, resp.StatusCode(), "status code mismatch")

			if tt.want.body != "" {
				assert.Contains(t, resp.String(), tt.want.body, "response should contain short URL prefix")
				assert.Contains(t, resp.String(), `"result":"`, "should have result field")
			}
		})
	}
}

func TestBatchShortenHandler(t *testing.T) {
	ts, mockRepo := setupTestServer(t)
	defer ts.Close()

	client := resty.New()
	client.SetBaseURL(ts.URL)

	tests := []struct {
		name          string
		items         []service.BatchItem
		wantStatus    int
		wantLen       int
		setupMocks    func()
		checkResponse func(*testing.T, *resty.Response)
	}{
		{
			name: "positive batch — two valid urls",
			items: []service.BatchItem{
				{CorrelationID: "id-1", OriginalURL: "https://ya.ru"},
				{CorrelationID: "id-2", OriginalURL: "https://google.com"},
			},
			wantStatus: http.StatusCreated,
			wantLen:    2,
			setupMocks: func() {
				mockRepo.EXPECT().
					Get(gomock.Any()).
					Return("", false).
					Times(2)

				mockRepo.EXPECT().
					BatchSave(gomock.Any(), gomock.Len(2)).
					Return(nil).
					Times(1)
			},
			checkResponse: func(t *testing.T, resp *resty.Response) {
				var results []service.BatchResult
				err := json.Unmarshal(resp.Body(), &results)
				require.NoError(t, err)

				assert.Len(t, results, 2)

				idsFound := make(map[string]bool)
				for _, r := range results {
					assert.NotEmpty(t, r.CorrelationID)
					assert.NotEmpty(t, r.ShortURL)
					assert.True(t, strings.HasPrefix(r.ShortURL, testBaseURL+"/"))
					idsFound[r.CorrelationID] = true
				}

				assert.True(t, idsFound["id-1"])
				assert.True(t, idsFound["id-2"])
			},
		},

		{
			name:          "empty batch",
			items:         []service.BatchItem{},
			wantStatus:    http.StatusBadRequest,
			wantLen:       0,
			setupMocks:    func() {},
			checkResponse: func(t *testing.T, resp *resty.Response) {},
		},

		{
			name: "invalid item",
			items: []service.BatchItem{
				{CorrelationID: "bad", OriginalURL: ""},
			},
			wantStatus:    http.StatusBadRequest,
			wantLen:       0,
			setupMocks:    func() {},
			checkResponse: func(t *testing.T, resp *resty.Response) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()

			resp, err := client.R().
				SetHeader("Content-Type", "application/json").
				SetBody(tt.items).
				Post("/api/shorten/batch")

			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, resp.StatusCode())

			if tt.wantStatus == http.StatusCreated {
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestGetHandler(t *testing.T) {
	ts, mockRepo := setupTestServer(t)
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
				shortID := "test123"
				mockRepo.EXPECT().Get(shortID).Return(tt.want.location, true).Times(1)
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

func TestPingHandler(t *testing.T) {
	ts, mockRepo := setupTestServer(t)
	defer ts.Close()

	client := resty.New()
	client.SetBaseURL(ts.URL)

	tests := []struct {
		name     string
		mockErr  error
		wantCode int
	}{
		{
			name:     "successful ping",
			mockErr:  nil,
			wantCode: http.StatusOK,
		},
		{
			name:     "ping error",
			mockErr:  fmt.Errorf("db error"),
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo.EXPECT().Ping(gomock.Any()).Return(tt.mockErr).Times(1)

			resp, err := client.R().Get("/ping")
			require.NoError(t, err)

			assert.Equal(t, tt.wantCode, resp.StatusCode())
		})
	}
}
