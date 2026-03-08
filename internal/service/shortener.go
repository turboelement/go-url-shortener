package service

import (
	"context"
	"errors"
	"math/rand"

	"go-url-shortener/internal/repository"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

const shortIDLength = 8

type ShortenerService struct {
	repo repository.URLRepositoryInterface
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
	return &ShortenerService{repo: repo}
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
		_, exists := s.repo.Get(shortID)
		if !exists {
			break
		}
		shortID = s.GenerateShortID()
	}

	s.repo.Save(shortID, originalURL)
	return shortID, nil
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
			_, exists := s.repo.Get(shortID)
			if !exists {
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

	if err := s.repo.BatchSave(ctx, batchEntries); err != nil {
		return nil, err
	}

	return results, nil
}

func (s *ShortenerService) GetOriginalURL(shortID string) (string, bool) {
	return s.repo.Get(shortID)
}
