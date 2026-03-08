package repository

import "context"

type URLRepositoryInterface interface {
	Save(shortID, originalURL string)
	Get(shortID string) (string, bool)
	Ping(ctx context.Context) error
	BatchSave(ctx context.Context, items []BatchEntry) error
}

type BatchEntry struct {
	ShortID     string
	OriginalURL string
}
