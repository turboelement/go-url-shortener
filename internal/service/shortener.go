package service

import (
	"math/rand"

	"go-url-shortener/internal/repository"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

const shortIDLength = 8

type ShortenerService struct {
	repo *repository.URLRepository
}

func NewShortenerService(repo *repository.URLRepository) *ShortenerService {
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

func (s *ShortenerService) GetOriginalURL(shortID string) (string, bool) {
	return s.repo.Get(shortID)
}
