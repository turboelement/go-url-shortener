// Package grpcserver implements the gRPC server for the URL shortener service.
// It acts as a facade over the shared ShortenerService business logic.
package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"go-url-shortener/api/proto/shortenerpb"
	"go-url-shortener/internal/repository"
	"go-url-shortener/internal/service"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ShortenerGRPCServer implements the shortenerpb.ShortenerServiceServer interface.
type ShortenerGRPCServer struct {
	shortenerpb.UnimplementedShortenerServiceServer
	svc     *service.ShortenerService
	baseURL string
}

// NewShortenerGRPCServer creates a new gRPC server with the given service and base URL.
func NewShortenerGRPCServer(svc *service.ShortenerService, baseURL string) *ShortenerGRPCServer {
	return &ShortenerGRPCServer{
		svc:     svc,
		baseURL: baseURL,
	}
}

// ShortenURL handles a URL shortening request.
// Counterpart of the HTTP handler POST /api/shorten.
func (s *ShortenerGRPCServer) ShortenURL(ctx context.Context, req *shortenerpb.URLShortenRequest) (*shortenerpb.URLShortenResponse, error) {
	originalURL := req.GetUrl()
	if originalURL == "" {
		return nil, status.Error(codes.InvalidArgument, "url is required")
	}

	userID, err := GetUserIDFromGRPCContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "cannot get user ID: %v", err)
	}

	shortID, err := s.svc.ShortenWithUser(ctx, originalURL, userID)
	if err != nil && !errors.Is(err, repository.ErrURLAlreadyExists) {
		return nil, status.Errorf(codes.Internal, "failed to shorten URL: %v", err)
	}

	// Save original error before url.JoinPath overwrites err
	alreadyExists := errors.Is(err, repository.ErrURLAlreadyExists)

	shortURL, joinErr := url.JoinPath(s.baseURL, shortID)
	if joinErr != nil {
		return nil, status.Errorf(codes.Internal, "failed to build short URL: %v", joinErr)
	}

	resp := &shortenerpb.URLShortenResponse{
		Result: shortURL,
	}

	// If URL already exists, return with AlreadyExists code
	if alreadyExists {
		return resp, status.Errorf(codes.AlreadyExists, "url already exists")
	}

	return resp, nil
}

// ExpandURL handles a request to retrieve the original URL by its short ID.
// Counterpart of the HTTP handler GET /{id}.
func (s *ShortenerGRPCServer) ExpandURL(ctx context.Context, req *shortenerpb.URLExpandRequest) (*shortenerpb.URLExpandResponse, error) {
	shortID := req.GetId()
	if shortID == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	originalURL, err := s.svc.GetOriginalURL(ctx, shortID)
	if err != nil {
		if errors.Is(err, repository.ErrURLMarkedAsDeleted) {
			return nil, status.Errorf(codes.NotFound, "url marked as deleted")
		}
		if errors.Is(err, repository.ErrURLNotFound) {
			return nil, status.Errorf(codes.NotFound, "short URL not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get original URL: %v", err)
	}

	if originalURL == "" {
		return nil, status.Errorf(codes.NotFound, "short URL not found")
	}

	resp := &shortenerpb.URLExpandResponse{
		Result: originalURL,
	}

	return resp, nil
}

// ListUserURLs returns all URLs belonging to the current user.
// Counterpart of the HTTP handler GET /api/user/urls.
func (s *ShortenerGRPCServer) ListUserURLs(ctx context.Context, _ *emptypb.Empty) (*shortenerpb.UserURLsResponse, error) {
	userID, err := GetUserIDFromGRPCContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "cannot get user ID: %v", err)
	}

	userURLs, err := s.svc.GetUserURLs(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user URLs: %v", err)
	}

	resp := &shortenerpb.UserURLsResponse{
		Url: make([]*shortenerpb.URLData, 0, len(userURLs)),
	}

	for _, u := range userURLs {
		shortURL, err := url.JoinPath(s.baseURL, u.ShortURL)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to build short URL: %v", err)
		}

		resp.Url = append(resp.Url, &shortenerpb.URLData{
			ShortUrl:    shortURL,
			OriginalUrl: u.OriginalURL,
		})
	}

	return resp, nil
}

// Validate checks that the server is configured correctly.
func (s *ShortenerGRPCServer) Validate() error {
	if s.svc == nil {
		return fmt.Errorf("service is nil")
	}
	if s.baseURL == "" {
		return fmt.Errorf("base URL is empty")
	}
	_, err := url.Parse(s.baseURL)
	if err != nil {
		return fmt.Errorf("base URL is invalid: %w", err)
	}
	return nil
}
