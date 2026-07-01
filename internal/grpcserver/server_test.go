package grpcserver

import (
	"context"
	"net"
	"strings"
	"testing"

	"go-url-shortener/api/proto/shortenerpb"
	"go-url-shortener/internal/repository"
	"go-url-shortener/internal/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

// grpcTestFixture holds shared test dependencies.
type grpcTestFixture struct {
	client   shortenerpb.ShortenerServiceClient
	conn     *grpc.ClientConn
	grpcSrv  *grpc.Server
	listener *bufconn.Listener
}

// setupGrpcTest creates an in-memory gRPC server and returns a fixture.
func setupGrpcTest(t testing.TB, secretKey string) *grpcTestFixture {
	t.Helper()

	repo := repository.NewURLRepository()
	svc := service.NewShortenerService(repo)
	grpcServer := NewShortenerGRPCServer(svc, "http://localhost:8080")

	if err := grpcServer.Validate(); err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}

	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			AuthInterceptor(secretKey),
		),
	)
	shortenerpb.RegisterShortenerServiceServer(srv, grpcServer)

	listener := bufconn.Listen(1024 * 1024)

	go func() {
		_ = srv.Serve(listener)
	}()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to create gRPC client: %v", err)
	}

	return &grpcTestFixture{
		client:   shortenerpb.NewShortenerServiceClient(conn),
		conn:     conn,
		grpcSrv:  srv,
		listener: listener,
	}
}

func (f *grpcTestFixture) close() {
	f.conn.Close()
	f.grpcSrv.GracefulStop()
	f.listener.Close()
}

// shortenAndGetToken creates a URL and returns the auth token from response headers.
func (f *grpcTestFixture) shortenAndGetToken(t testing.TB, url string) (shortID, token string) {
	t.Helper()

	var header metadata.MD
	resp, err := f.client.ShortenURL(
		context.Background(),
		(&shortenerpb.URLShortenRequest_builder{Url: url}).Build(),
		grpc.Header(&header),
	)
	if err != nil {
		t.Fatalf("ShortenURL(%q) failed: %v", url, err)
	}

	shortID = strings.TrimPrefix(resp.GetResult(), "http://localhost:8080/")
	token = header.Get("user_id")[0]
	return
}

// authedContext attaches an auth token to the context.
func authedContext(token string) context.Context {
	return metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs("authorization", token),
	)
}

func TestShortenURL(t *testing.T) {
	const secretKey = "test-secret"
	f := setupGrpcTest(t, secretKey)
	defer f.close()

	tests := []struct {
		name          string
		url           string
		wantErr       string // substring of expected error; empty means success
		wantDuplicate bool
	}{
		{
			name: "valid URL",
			url:  "https://example.com/alpha",
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: "InvalidArgument",
		},
	}

	// Reserve one URL for the duplicate test
	var dupHeader metadata.MD
	_, err := f.client.ShortenURL(
		context.Background(),
		(&shortenerpb.URLShortenRequest_builder{Url: "https://example.com/dup"}).Build(),
		grpc.Header(&dupHeader),
	)
	if err != nil {
		t.Fatalf("failed to create duplicate test URL: %v", err)
	}
	tests = append(tests, struct {
		name          string
		url           string
		wantErr       string
		wantDuplicate bool
	}{
		name:          "duplicate URL",
		url:           "https://example.com/dup",
		wantDuplicate: true,
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var header metadata.MD
			resp, err := f.client.ShortenURL(
				context.Background(),
				(&shortenerpb.URLShortenRequest_builder{Url: tt.url}).Build(),
				grpc.Header(&header),
			)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify duplicate flag
			if tt.wantDuplicate {
				if !resp.GetAlreadyExists() {
					t.Fatal("expected AlreadyExists flag to be true for duplicate URL")
				}
				if resp.GetResult() == "" {
					t.Fatal("expected non-empty short URL even for duplicate")
				}
				return
			}

			// Verify auth header is set for new users
			authValues := header.Get("user_id")
			if len(authValues) == 0 {
				t.Fatal("expected user_id in response header for new user")
			}
		})
	}
}

func TestExpandURL(t *testing.T) {
	const secretKey = "test-secret"
	f := setupGrpcTest(t, secretKey)
	defer f.close()

	// Create a URL to expand later
	shortID, token := f.shortenAndGetToken(t, "https://example.com/target")

	tests := []struct {
		name    string
		shortID string
		token   string
		wantURL string
		wantErr string // substring of expected error
	}{
		{
			name:    "valid short ID",
			shortID: shortID,
			token:   token,
			wantURL: "https://example.com/target",
		},
		{
			name:    "empty ID",
			shortID: "",
			token:   token,
			wantErr: "InvalidArgument",
		},
		{
			name:    "non-existent ID",
			shortID: "nonexistent",
			token:   token,
			wantErr: "NotFound",
		},
		{
			name:    "invalid token",
			shortID: shortID,
			token:   "Bearer invalid-token",
			wantErr: "Unauthenticated",
		},
		{
			name:    "expired/malformed token",
			shortID: shortID,
			token:   "garbage",
			wantErr: "Unauthenticated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := authedContext(tt.token)
			resp, err := f.client.ExpandURL(ctx, (&shortenerpb.URLExpandRequest_builder{Id: tt.shortID}).Build())

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.GetResult() != tt.wantURL {
				t.Fatalf("expected %q, got %q", tt.wantURL, resp.GetResult())
			}
		})
	}
}

func TestListUserURLs(t *testing.T) {
	const secretKey = "test-secret"
	f := setupGrpcTest(t, secretKey)
	defer f.close()

	// Create URLs for two separate users
	var user1Header metadata.MD
	_, err := f.client.ShortenURL(
		context.Background(),
		(&shortenerpb.URLShortenRequest_builder{Url: "https://example.com/user1-1"}).Build(),
		grpc.Header(&user1Header),
	)
	if err != nil {
		t.Fatalf("failed to create user1-1: %v", err)
	}
	tokenUser1 := user1Header.Get("user_id")[0]

	// Second URL for same user (reuse token)
	_, err = f.client.ShortenURL(
		authedContext(tokenUser1),
		(&shortenerpb.URLShortenRequest_builder{Url: "https://example.com/user1-2"}).Build(),
	)
	if err != nil {
		t.Fatalf("failed to create user1-2: %v", err)
	}

	// Second user
	var user2Header metadata.MD
	_, err = f.client.ShortenURL(
		context.Background(),
		(&shortenerpb.URLShortenRequest_builder{Url: "https://example.com/user2-1"}).Build(),
		grpc.Header(&user2Header),
	)
	if err != nil {
		t.Fatalf("failed to create user2-1: %v", err)
	}
	tokenUser2 := user2Header.Get("user_id")[0]

	tests := []struct {
		name     string
		token    string
		wantURLs []string // expected original URLs
		wantErr  string   // substring of expected error
	}{
		{
			name:     "user with multiple URLs",
			token:    tokenUser1,
			wantURLs: []string{"https://example.com/user1-1", "https://example.com/user1-2"},
		},
		{
			name:     "user with single URL",
			token:    tokenUser2,
			wantURLs: []string{"https://example.com/user2-1"},
		},
		{
			name:    "invalid token",
			token:   "Bearer invalid-token",
			wantErr: "Unauthenticated",
		},
		{
			name:    "malformed token",
			token:   "garbage",
			wantErr: "Unauthenticated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := authedContext(tt.token)
			resp, err := f.client.ListUserURLs(ctx, &shortenerpb.ListUserURLsRequest{})

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(resp.GetUrl()) != len(tt.wantURLs) {
				t.Fatalf("expected %d URLs, got %d", len(tt.wantURLs), len(resp.GetUrl()))
			}

			for i, u := range resp.GetUrl() {
				if u.GetOriginalUrl() != tt.wantURLs[i] {
					t.Errorf("URL #%d: expected %q, got %q", i, tt.wantURLs[i], u.GetOriginalUrl())
				}
				// Verify short URL is properly prefixed
				if !strings.HasPrefix(u.GetShortUrl(), "http://localhost:8080/") {
					t.Errorf("URL #%d: short URL missing base prefix: %s", i, u.GetShortUrl())
				}
			}
		})
	}
}
