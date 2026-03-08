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
	BatchSave(ctx context.Context, items []BatchEntry) error
}

type BatchEntry struct {
	ShortID     string
	OriginalURL string
}

var ErrURLAlreadyExists = errors.New("url already exists")
