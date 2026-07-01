// Package grpcserver implements the gRPC server for the URL shortener service.
// It acts as a facade over the shared ShortenerService business logic.
package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"go-url-shortener/api/proto/shortenerpb"
	"go-url-shortener/internal/logger"
	"go-url-shortener/internal/repository"
	"go-url-shortener/internal/service"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	userID, err := getUserIDFromGRPCContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "cannot get user ID: %v", err)
	}

	shortID, err := s.svc.ShortenWithUser(ctx, originalURL, userID)
	if err != nil && !errors.Is(err, repository.ErrURLAlreadyExists) {
		logger.FromContext(ctx).Error("failed to shorten URL", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal server error")
	}

	// Save original error before url.JoinPath overwrites err
	alreadyExists := errors.Is(err, repository.ErrURLAlreadyExists)

	shortURL, joinErr := url.JoinPath(s.baseURL, shortID)
	if joinErr != nil {
		logger.FromContext(ctx).Error("failed to build short URL", zap.Error(joinErr))
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return (&shortenerpb.URLShortenResponse_builder{
		Result:        shortURL,
		AlreadyExists: alreadyExists,
	}).Build(), nil
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
		logger.FromContext(ctx).Error("failed to get original URL", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal server error")
	}

	if originalURL == "" {
		return nil, status.Errorf(codes.NotFound, "short URL not found")
	}

	resp := (&shortenerpb.URLExpandResponse_builder{Result: originalURL}).Build()

	return resp, nil
}

// ListUserURLs returns all URLs belonging to the current user.
// Counterpart of the HTTP handler GET /api/user/urls.
func (s *ShortenerGRPCServer) ListUserURLs(ctx context.Context, _ *shortenerpb.ListUserURLsRequest) (*shortenerpb.UserURLsResponse, error) {
	userID, err := getUserIDFromGRPCContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "cannot get user ID: %v", err)
	}

	userURLs, err := s.svc.GetUserURLs(ctx, userID)
	if err != nil {
		logger.FromContext(ctx).Error("failed to get user URLs", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal server error")
	}

	resp := (&shortenerpb.UserURLsResponse_builder{Url: make([]*shortenerpb.URLData, 0, len(userURLs))}).Build()

	for _, u := range userURLs {
		shortURL, joinErr := url.JoinPath(s.baseURL, u.ShortURL)
		if joinErr != nil {
			logger.FromContext(ctx).Error("failed to build short URL", zap.Error(joinErr))
			return nil, status.Error(codes.Internal, "internal server error")
		}

		resp.SetUrl(append(resp.GetUrl(), (&shortenerpb.URLData_builder{
			ShortUrl:    shortURL,
			OriginalUrl: u.OriginalURL,
		}).Build()))
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
