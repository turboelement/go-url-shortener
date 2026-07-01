// Package repository provides storage interfaces and implementations for URLs.
package repository

import (
	"context"
	"errors"
)

// URLRepositoryInterface defines all storage operations for URLs.
//
//go:generate mockgen --source=repository.go  --destination=mocks/mock_repository.go --package=mocks URLRepositoryInterface
type URLRepositoryInterface interface {
	Save(ctx context.Context, shortID, originalURL string) (string, error)
	Get(ctx context.Context, shortID string) (string, error)
	Ping(ctx context.Context) error
	BatchSave(ctx context.Context, userID string, items []BatchEntry) error
	SaveWithUser(ctx context.Context, shortID, originalURL, userID string) (string, error)
	GetUserURLs(ctx context.Context, userID string) ([]UserURL, error)
	DeleteUserURLs(ctx context.Context, userID string, shortIDs []string) error
	Stats(ctx context.Context) (StatsResult, error)
}

// BatchEntry is a single item to store in a batch save operation.
type BatchEntry struct {
	ShortID     string
	OriginalURL string
}

// UserURL represents a short↔original URL pair belonging to a user.
type UserURL struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// StatsResult contains aggregate statistics for the service.
type StatsResult struct {
	URLs  int `json:"urls"`
	Users int `json:"users"`
}

// ErrURLAlreadyExists is returned when trying to save a duplicate URL.
var ErrURLAlreadyExists = errors.New("url already exists")

// ErrURLMarkedAsDeleted is returned when the URL has been soft-deleted.
var ErrURLMarkedAsDeleted = errors.New("url marked as deleted")

// ErrURLNotFound is returned when a short ID does not exist.
var ErrURLNotFound = errors.New("url not found")
