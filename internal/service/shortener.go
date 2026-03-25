package service

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"go-url-shortener/internal/logger"
	"go-url-shortener/internal/repository"

	"go.uber.org/zap"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

const shortIDLength = 8

type deleteTask struct {
	UserID   string
	ShortIDs []string
}

type ShortenerService struct {
	repo     repository.URLRepositoryInterface
	deleteCh chan deleteTask
}

type BatchItem struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

type BatchResult struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

func NewShortenerService(repo repository.URLRepositoryInterface) *ShortenerService {
	s := &ShortenerService{
		repo:     repo,
		deleteCh: make(chan deleteTask, 1000),
	}

	go s.deleteWorker()

	return s
}

func (s *ShortenerService) GenerateShortID() string {
	b := make([]rune, shortIDLength)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func (s *ShortenerService) Shorten(originalURL string) (string, error) {
	// TODO: validate url
	shortID := s.GenerateShortID()

	for {
		_, err := s.repo.Get(shortID)
		if err == repository.ErrURLNotFound {
			break
		}
		shortID = s.GenerateShortID()
	}

	storedShortID, err := s.repo.Save(shortID, originalURL)
	if err != nil {
		if errors.Is(err, repository.ErrURLAlreadyExists) {
			return storedShortID, repository.ErrURLAlreadyExists
		}
		return "", err
	}

	return storedShortID, nil
}

func (s *ShortenerService) ShortenWithUser(originalURL, userID string) (string, error) {
	// TODO: validate url
	shortID := s.GenerateShortID()

	for {
		_, err := s.repo.Get(shortID)
		if err == repository.ErrURLNotFound {
			break
		}
		shortID = s.GenerateShortID()
	}

	storedShortID, err := s.repo.SaveWithUser(shortID, originalURL, userID)
	if err != nil {
		if errors.Is(err, repository.ErrURLAlreadyExists) {
			return storedShortID, repository.ErrURLAlreadyExists
		}
		return "", err
	}

	return storedShortID, nil
}

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
			_, err := s.repo.Get(shortID)
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
			_, err := s.repo.Get(shortID)
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

func (s *ShortenerService) GetOriginalURL(shortID string) (string, error) {
	return s.repo.Get(shortID)
}

func (s *ShortenerService) GetUserURLs(userID string) ([]repository.UserURL, error) {
	return s.repo.GetUserURLs(userID)
}

func (s *ShortenerService) DeleteUserURLs(ctx context.Context, userID string, shortIDs []string) error {
	return s.repo.DeleteUserURLs(ctx, userID, shortIDs)
}

func (s *ShortenerService) DeleteUserURLsAsync(userID string, shortIDs []string) {
	if len(shortIDs) == 0 {
		return
	}
	select {
	case s.deleteCh <- deleteTask{UserID: userID, ShortIDs: shortIDs}:
	default:
	}
}

func (s *ShortenerService) deleteWorker() {
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
		}
	}
}

func (s *ShortenerService) flush(tasks []deleteTask) {
	if len(tasks) == 0 {
		return
	}
	ctx := context.Background()

	for _, t := range tasks {
		if err := s.repo.DeleteUserURLs(ctx, t.UserID, t.ShortIDs); err != nil {
			logger.FromContext(ctx).Error("delete worker error", zap.Error(err))
		}
	}
}
