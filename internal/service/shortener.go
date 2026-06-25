// Package service contains business logic for URL shortening.
package service

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"go-url-shortener/internal/logger"
	"go-url-shortener/internal/repository"

	"go.uber.org/zap"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

const shortIDLength = 8

type deleteTask struct {
	UserID   string
	ShortIDs []string
}

// ShortenerService provides URL shortening operations.
type ShortenerService struct {
	repo     repository.URLRepositoryInterface
	deleteCh chan deleteTask
	doneCh   chan struct{}
	wg       sync.WaitGroup
	closed   bool
	mu       sync.RWMutex
}

// BatchItem is a single URL item in a batch shorten request.
type BatchItem struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

// BatchResult contains the short URL and its correlation ID from a batch request.
type BatchResult struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

// NewShortenerService creates a service with the given repository and starts the async delete worker.
func NewShortenerService(repo repository.URLRepositoryInterface) *ShortenerService {
	s := &ShortenerService{
		repo:     repo,
		deleteCh: make(chan deleteTask, 1000),
		doneCh:   make(chan struct{}),
	}

	s.wg.Add(1)
	go s.deleteWorker()

	return s
}

// GenerateShortID returns a random 8-character alphanumeric ID.
func (s *ShortenerService) GenerateShortID() string {
	b := make([]byte, shortIDLength)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

// Shorten saves a URL and returns its short ID.
func (s *ShortenerService) Shorten(ctx context.Context, originalURL string) (string, error) {
	shortID := s.GenerateShortID()

	for {
		_, err := s.repo.Get(ctx, shortID)
		if err == repository.ErrURLNotFound {
			break
		}
		shortID = s.GenerateShortID()
	}

	storedShortID, err := s.repo.Save(ctx, shortID, originalURL)
	if err != nil {
		if errors.Is(err, repository.ErrURLAlreadyExists) {
			return storedShortID, repository.ErrURLAlreadyExists
		}
		return "", err
	}

	return storedShortID, nil
}

// ShortenWithUser saves a URL linked to a user and returns its short ID.
func (s *ShortenerService) ShortenWithUser(ctx context.Context, originalURL, userID string) (string, error) {
	shortID := s.GenerateShortID()

	for {
		_, err := s.repo.Get(ctx, shortID)
		if err == repository.ErrURLNotFound {
			break
		}
		shortID = s.GenerateShortID()
	}

	storedShortID, err := s.repo.SaveWithUser(ctx, shortID, originalURL, userID)
	if err != nil {
		if errors.Is(err, repository.ErrURLAlreadyExists) {
			return storedShortID, repository.ErrURLAlreadyExists
		}
		return "", err
	}

	return storedShortID, nil
}

// BatchShorten shortens multiple URLs in one call (no user association).
func (s *ShortenerService) BatchShorten(ctx context.Context, items []BatchItem) ([]BatchResult, error) {
	if len(items) == 0 {
		return nil, errors.New("empty batch")
	}

	batchEntries := make([]repository.BatchEntry, 0, len(items))
	results := make([]BatchResult, 0, len(items))

	for _, item := range items {
		if item.OriginalURL == "" || item.CorrelationID == "" {
			return nil, errors.New("invalid item")
		}

		shortID := s.GenerateShortID()
		for {
			_, err := s.repo.Get(ctx, shortID)
			if err == repository.ErrURLNotFound {
				break
			}
			shortID = s.GenerateShortID()
		}

		batchEntries = append(batchEntries, repository.BatchEntry{
			ShortID:     shortID,
			OriginalURL: item.OriginalURL,
		})

		results = append(results, BatchResult{
			CorrelationID: item.CorrelationID,
			ShortURL:      shortID,
		})
	}

	if err := s.repo.BatchSave(ctx, "", batchEntries); err != nil {
		return nil, err
	}

	return results, nil
}

// BatchShortenWithUser shortens multiple URLs linked to a user.
func (s *ShortenerService) BatchShortenWithUser(ctx context.Context, userID string, items []BatchItem) ([]BatchResult, error) {
	if len(items) == 0 {
		return nil, errors.New("empty batch")
	}

	batchEntries := make([]repository.BatchEntry, 0, len(items))
	results := make([]BatchResult, 0, len(items))

	for _, item := range items {
		if item.OriginalURL == "" || item.CorrelationID == "" {
			return nil, errors.New("invalid item")
		}

		shortID := s.GenerateShortID()
		for {
			_, err := s.repo.Get(ctx, shortID)
			if err == repository.ErrURLNotFound {
				break
			}
			shortID = s.GenerateShortID()
		}

		batchEntries = append(batchEntries, repository.BatchEntry{
			ShortID:     shortID,
			OriginalURL: item.OriginalURL,
		})

		results = append(results, BatchResult{
			CorrelationID: item.CorrelationID,
			ShortURL:      shortID,
		})
	}

	if err := s.repo.BatchSave(ctx, userID, batchEntries); err != nil {
		return nil, err
	}

	return results, nil
}

// GetOriginalURL returns the original URL for a short ID.
func (s *ShortenerService) GetOriginalURL(ctx context.Context, shortID string) (string, error) {
	return s.repo.Get(ctx, shortID)
}

// GetStats returns the total number of URLs and unique users in the system.
func (s *ShortenerService) GetStats(ctx context.Context) (int, int, error) {
	return s.repo.Stats(ctx)
}

// GetUserURLs returns all URLs created by the given user.
func (s *ShortenerService) GetUserURLs(ctx context.Context, userID string) ([]repository.UserURL, error) {
	return s.repo.GetUserURLs(ctx, userID)
}

// DeleteUserURLs marks the specified URLs as deleted for the given user (synchronous).
func (s *ShortenerService) DeleteUserURLs(ctx context.Context, userID string, shortIDs []string) error {
	return s.repo.DeleteUserURLs(ctx, userID, shortIDs)
}

// DeleteUserURLsAsync sends URLs to the async delete worker for batch deletion.
func (s *ShortenerService) DeleteUserURLsAsync(userID string, shortIDs []string) {
	if len(shortIDs) == 0 {
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return
	}

	select {
	case s.deleteCh <- deleteTask{UserID: userID, ShortIDs: shortIDs}:
	default:
	}
}

func (s *ShortenerService) deleteWorker() {
	defer s.wg.Done()

	const (
		maxBatchSize  = 200
		flushInterval = 5 * time.Second
	)

	var batch []deleteTask
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case task, ok := <-s.deleteCh:
			if !ok {
				s.flush(batch)
				return
			}
			batch = append(batch, task)

			if len(batch) >= maxBatchSize {
				s.flush(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				s.flush(batch)
				batch = batch[:0]
			}

		case <-s.doneCh:
			s.flush(batch)
			return
		}
	}
}

func (s *ShortenerService) flush(tasks []deleteTask) {
	if len(tasks) == 0 {
		return
	}
	ctx := context.Background()

	userBatch := make(map[string][]string, len(tasks))
	for _, t := range tasks {
		userBatch[t.UserID] = append(userBatch[t.UserID], t.ShortIDs...)
	}

	for userID, shortIDs := range userBatch {
		if err := s.repo.DeleteUserURLs(ctx, userID, shortIDs); err != nil {
			logger.FromContext(ctx).Error("delete worker error", zap.Error(err))
		}
	}
}

// Close gracefully stops the async delete worker and releases resources.
func (s *ShortenerService) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	close(s.doneCh)

	s.wg.Wait()

	close(s.deleteCh)

	return nil
}
