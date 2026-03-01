package repository

import "context"

type URLRepositoryInterface interface {
	Save(shortID, originalURL string)
	Get(shortID string) (string, bool)
	Ping(ctx context.Context) error
}
