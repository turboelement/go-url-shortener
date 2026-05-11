package repository

import (
	"context"
	"sync"
)

// URLEntry stores a single URL record in memory.
type URLEntry struct {
	ShortID     string
	OriginalURL string
	UserID      string
	DeletedFlag bool
}

// URLRepository is an in-memory implementation of URLRepositoryInterface.
type URLRepository struct {
	store     map[string]URLEntry // shortID | URLEntry (value, not pointer — reduces GC pressure)
	rev       map[string]string   // reverse originalURL | shortID
	userIndex map[string][]string // userID | []shortID
	mu        sync.RWMutex
}

// NewURLRepository creates an empty in-memory URL repository.
func NewURLRepository() *URLRepository {
	return &URLRepository{
		store:     make(map[string]URLEntry),
		rev:       make(map[string]string),
		userIndex: make(map[string][]string),
		// mu no need to init — zero value sync.RWMutex is ready to use
	}
}

// Save stores a URL and returns its short ID. Returns ErrURLAlreadyExists if duplicate.
func (r *URLRepository) Save(ctx context.Context, shortID, originalURL string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if storedID, exists := r.rev[originalURL]; exists {
		return storedID, ErrURLAlreadyExists
	}

	r.store[shortID] = URLEntry{
		ShortID:     shortID,
		OriginalURL: originalURL,
	}
	r.rev[originalURL] = shortID

	return shortID, nil
}

// Get returns the original URL for a short ID. Thread-safe.
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

// Ping always returns nil (in-memory is always available).
func (r *URLRepository) Ping(ctx context.Context) error {
	return nil
}

// SaveWithUser stores a URL linked to a user. Returns ErrURLAlreadyExists if duplicate.
func (r *URLRepository) SaveWithUser(ctx context.Context, shortID, originalURL, userID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if storedID, exists := r.rev[originalURL]; exists {
		return storedID, ErrURLAlreadyExists
	}

	r.store[shortID] = URLEntry{
		ShortID:     shortID,
		OriginalURL: originalURL,
		UserID:      userID,
	}
	r.rev[originalURL] = shortID

	r.userIndex[userID] = append(r.userIndex[userID], shortID)

	return shortID, nil
}

// GetUserURLs returns all non-deleted URLs for a given user.
func (r *URLRepository) GetUserURLs(ctx context.Context, userID string) ([]UserURL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	shortIDs, exists := r.userIndex[userID]
	if !exists || len(shortIDs) == 0 {
		return []UserURL{}, nil
	}

	result := make([]UserURL, 0, len(shortIDs))
	for _, shortID := range shortIDs {
		entry, ok := r.store[shortID]
		if ok && !entry.DeletedFlag {
			result = append(result, UserURL{
				ShortURL:    entry.ShortID,
				OriginalURL: entry.OriginalURL,
			})
		}
	}

	return result, nil
}

// BatchSave stores multiple URL entries atomically.
func (r *URLRepository) BatchSave(ctx context.Context, userID string, items []BatchEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, item := range items {
		if _, exists := r.rev[item.OriginalURL]; !exists {
			r.store[item.ShortID] = URLEntry{
				ShortID:     item.ShortID,
				OriginalURL: item.OriginalURL,
				UserID:      userID,
			}
			r.rev[item.OriginalURL] = item.ShortID

			if userID != "" {
				r.userIndex[userID] = append(r.userIndex[userID], item.ShortID)
			}
		}
	}

	return nil
}

// DeleteUserURLs soft-deletes the specified URLs owned by the user.
func (r *URLRepository) DeleteUserURLs(ctx context.Context, userID string, shortIDs []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, shortID := range shortIDs {
		if entry, ok := r.store[shortID]; ok && entry.UserID == userID {
			entry.DeletedFlag = true
			r.store[shortID] = entry
		}
	}

	return nil
}
