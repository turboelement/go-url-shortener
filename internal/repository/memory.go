package repository

import (
	"context"
	"sync"
)

type URLEntry struct {
	ShortID     string
	OriginalURL string
	UserID      string
	DeletedFlag bool
}

type URLRepository struct {
	store map[string]*URLEntry // shortID | URLEntry
	rev   map[string]string    // reverse originalURL | shortID
	mu    sync.RWMutex
}

func NewURLRepository() *URLRepository {
	return &URLRepository{
		store: make(map[string]*URLEntry),
		rev:   make(map[string]string),
		// mu no need to init — zero value sync.RWMutex is ready to use
	}
}

func (r *URLRepository) Save(ctx context.Context, shortID, originalURL string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if storedID, exists := r.rev[originalURL]; exists {
		return storedID, ErrURLAlreadyExists
	}

	r.store[shortID] = &URLEntry{
		ShortID:     shortID,
		OriginalURL: originalURL,
	}
	r.rev[originalURL] = shortID

	return shortID, nil
}

func (r *URLRepository) Get(ctx context.Context, shortID string) (string, error) {
	r.mu.RLock() // allows parralel Get
	defer r.mu.RUnlock()

	entry, ok := r.store[shortID]
	if !ok {
		return "", ErrURLNotFound
	}
	if entry.DeletedFlag {
		return "", ErrURLMarkedAsDeleted
	}
	return entry.OriginalURL, nil
}

func (r *URLRepository) Ping(ctx context.Context) error {
	return nil
}

func (r *URLRepository) SaveWithUser(ctx context.Context, shortID, originalURL, userID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if storedID, exists := r.rev[originalURL]; exists {
		return storedID, ErrURLAlreadyExists
	}

	r.store[shortID] = &URLEntry{
		ShortID:     shortID,
		OriginalURL: originalURL,
		UserID:      userID,
	}
	r.rev[originalURL] = shortID

	return shortID, nil
}

func (r *URLRepository) GetUserURLs(ctx context.Context, userID string) ([]UserURL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]UserURL, 0)
	for _, entry := range r.store {
		if entry.UserID == userID && !entry.DeletedFlag {
			result = append(result, UserURL{
				ShortURL:    entry.ShortID,
				OriginalURL: entry.OriginalURL,
			})
		}
	}

	return result, nil
}

func (r *URLRepository) BatchSave(ctx context.Context, userID string, items []BatchEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, item := range items {
		if _, exists := r.rev[item.OriginalURL]; !exists {
			r.store[item.ShortID] = &URLEntry{
				ShortID:     item.ShortID,
				OriginalURL: item.OriginalURL,
				UserID:      userID,
			}
			r.rev[item.OriginalURL] = item.ShortID
		}
	}

	return nil
}

func (r *URLRepository) DeleteUserURLs(ctx context.Context, userID string, shortIDs []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, shortID := range shortIDs {
		if entry, ok := r.store[shortID]; ok && entry.UserID == userID {
			entry.DeletedFlag = true
		}
	}

	return nil
}
