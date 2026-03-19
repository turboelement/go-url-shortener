package repository

import (
	"context"
	"errors"
)

//go:generate mockgen --source=repository.go  --destination=mocks/mock_repository.go --package=mocks URLRepositoryInterface
type URLRepositoryInterface interface {
	Save(shortID, originalURL string) (string, error)
	Get(shortID string) (string, bool)
	Ping(ctx context.Context) error
	BatchSave(ctx context.Context, userID string, items []BatchEntry) error
	SaveWithUser(shortID, originalURL, userID string) (string, error)
	GetUserURLs(userID string) ([]UserURL, error)
}

type BatchEntry struct {
	ShortID     string
	OriginalURL string
}

type UserURL struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

var ErrURLAlreadyExists = errors.New("url already exists")
