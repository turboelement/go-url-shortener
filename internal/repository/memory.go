package repository

import (
	"context"
	"sync"
)

type URLRepository struct {
	store map[string]string // shortID | originalURL
	rev   map[string]string // reverse originalURL | shortID
	mu    sync.RWMutex
}

func NewURLRepository() *URLRepository {
	return &URLRepository{
		store: make(map[string]string),
		rev:   make(map[string]string),
		// mu no need to init — zero value sync.RWMutex is ready to use
	}
}

func (r *URLRepository) Save(shortID, originalURL string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if storedID, exists := r.rev[originalURL]; exists {
		return storedID, ErrURLAlreadyExists
	}

	r.store[shortID] = originalURL
	r.rev[originalURL] = shortID

	return shortID, nil
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

func (r *URLRepository) BatchSave(ctx context.Context, items []BatchEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, item := range items {
		if _, exists := r.rev[item.OriginalURL]; !exists {
			r.store[item.ShortID] = item.OriginalURL
			r.rev[item.OriginalURL] = item.ShortID
		}
	}

	return nil
}
