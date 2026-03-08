package repository

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
)

type FileURLRepository struct {
	*URLRepository // embedding in-memory repo
	filePath       string
	file           *os.File
}

type FileEntry struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

func NewFileURLRepository(filePath string) *FileURLRepository {
	repo := &FileURLRepository{
		URLRepository: NewURLRepository(),
		filePath:      filePath,
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("Error opening file: %s: %v\n", filePath, err)
	} else {
		repo.file = f
	}

	repo.loadFromFile()
	return repo
}

func (r *FileURLRepository) Save(shortID, originalURL string) (string, error) {
	storedID, err := r.URLRepository.Save(shortID, originalURL)
	if err != nil {
		if errors.Is(err, ErrURLAlreadyExists) {
			return storedID, err
		}
		return "", err
	}

	if r.file == nil {
		return storedID, nil
	}

	entry := FileEntry{
		UUID:        uuid.NewString(),
		ShortURL:    shortID,
		OriginalURL: originalURL,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("error marshaling to json: %w", err)
	}

	_, err = r.file.Write(append(data, '\n'))
	if err != nil {
		return "", fmt.Errorf("error writing to file: %w", err)
	}

	return storedID, nil
}

func (r *FileURLRepository) Ping(ctx context.Context) error {
	return nil
}

func (r *FileURLRepository) BatchSave(ctx context.Context, items []BatchEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, item := range items {
		if _, exists := r.rev[item.OriginalURL]; exists {
			continue
		}

		r.store[item.ShortID] = item.OriginalURL
		r.rev[item.OriginalURL] = item.ShortID

		if r.file != nil {
			entry := FileEntry{
				UUID:        uuid.NewString(),
				ShortURL:    item.ShortID,
				OriginalURL: item.OriginalURL,
			}

			data, err := json.Marshal(entry)
			if err != nil {
				return fmt.Errorf("error marshaling to json: %w", err)
			}

			_, err = r.file.Write(append(data, '\n'))
			if err != nil {
				return fmt.Errorf("error writing to file: %w", err)
			}
		}
	}

	return nil
}

func (r *FileURLRepository) loadFromFile() {
	f, err := os.Open(r.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Error reading file %s: %v\n", r.filePath, err)
		}
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	r.mu.Lock()
	defer r.mu.Unlock()

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry FileEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			fmt.Printf("Error unmarshaling line from file: %v\n", err)
			continue
		}

		if entry.ShortURL != "" && entry.OriginalURL != "" {
			r.store[entry.ShortURL] = entry.OriginalURL
			r.rev[entry.OriginalURL] = entry.ShortURL
		}
	}
}
