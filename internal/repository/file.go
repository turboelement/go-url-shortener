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

// FileURLRepository stores URLs in-memory and also persists them to a file.
type FileURLRepository struct {
	*URLRepository // embedding in-memory repo
	filePath       string
	file           *os.File
	bufWriter      *bufio.Writer
}

// FileEntry represents one line in the JSON file storage.
type FileEntry struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserID      string `json:"user_id,omitempty"`
	DeletedFlag bool   `json:"deleted_flag,omitempty"`
}

// NewFileURLRepository creates a file-backed repository, loading existing data from the file.
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
		repo.bufWriter = bufio.NewWriterSize(f, 32*1024)
	}

	repo.loadFromFile()
	return repo
}

// Stats returns the total number of URLs and unique users.
func (r *FileURLRepository) Stats(ctx context.Context) (StatsResult, error) {
	return r.URLRepository.Stats(ctx)
}

// Save stores a URL in memory and appends it to the file. Returns ErrURLAlreadyExists if duplicate.
func (r *FileURLRepository) Save(ctx context.Context, shortID, originalURL string) (string, error) {
	storedID, err := r.URLRepository.Save(ctx, shortID, originalURL)
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

	if err := r.writeLine(data); err != nil {
		return "", err
	}

	return storedID, nil
}

// SaveWithUser stores a user-linked URL in memory and appends it to the file.
func (r *FileURLRepository) SaveWithUser(ctx context.Context, shortID, originalURL, userID string) (string, error) {
	storedID, err := r.URLRepository.SaveWithUser(ctx, shortID, originalURL, userID)
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
		UserID:      userID,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("error marshaling to json: %w", err)
	}

	if err := r.writeLine(data); err != nil {
		return "", err
	}

	return storedID, nil
}

// Ping always returns nil (file repo uses in-memory storage).
func (r *FileURLRepository) Ping(ctx context.Context) error {
	return nil
}

// BatchSave stores multiple URLs atomically and writes them to the file.
func (r *FileURLRepository) BatchSave(ctx context.Context, userID string, items []BatchEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, item := range items {
		if _, exists := r.rev[item.OriginalURL]; exists {
			continue
		}

		r.store[item.ShortID] = URLEntry{
			ShortID:     item.ShortID,
			OriginalURL: item.OriginalURL,
			UserID:      userID,
		}
		r.rev[item.OriginalURL] = item.ShortID

		if userID != "" {
			r.userIndex[userID] = append(r.userIndex[userID], item.ShortID)
		}

		if r.file != nil {
			entry := FileEntry{
				UUID:        uuid.NewString(),
				ShortURL:    item.ShortID,
				OriginalURL: item.OriginalURL,
				UserID:      userID,
			}

			data, err := json.Marshal(entry)
			if err != nil {
				return fmt.Errorf("error marshaling to json: %w", err)
			}

			if err := r.writeLine(data); err != nil {
				return err
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
			r.store[entry.ShortURL] = URLEntry{
				ShortID:     entry.ShortURL,
				OriginalURL: entry.OriginalURL,
				UserID:      entry.UserID,
				DeletedFlag: entry.DeletedFlag,
			}
			r.rev[entry.OriginalURL] = entry.ShortURL

			if entry.UserID != "" {
				r.userIndex[entry.UserID] = append(r.userIndex[entry.UserID], entry.ShortURL)
			}
		}
	}
}

func (r *FileURLRepository) writeLine(data []byte) error {
	if r.bufWriter == nil {
		return nil
	}
	if _, err := r.bufWriter.Write(data); err != nil {
		return fmt.Errorf("error writing to buffer: %w", err)
	}
	if _, err := r.bufWriter.Write([]byte{'\n'}); err != nil {
		return fmt.Errorf("error writing newline: %w", err)
	}
	return r.bufWriter.Flush()
}
