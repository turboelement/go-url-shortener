package repository

import (
	"context"
	"sync"
)

type URLRepository struct {
	store map[string]string
	mu    sync.RWMutex
}

func NewURLRepository() *URLRepository {
	return &URLRepository{
		store: make(map[string]string),
		// mu no need to init — zero value sync.RWMutex is ready to use
	}
}

func (r *URLRepository) Save(shortID, originalURL string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.store[shortID] = originalURL
}

func (r *URLRepository) Get(shortID string) (string, bool) {
	r.mu.RLock() // allows parralel Get
	defer r.mu.RUnlock()

	url, ok := r.store[shortID]
	return url, ok
}

func (r *URLRepository) Ping(ctx context.Context) error {
	return nil
}
