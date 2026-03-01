package repository

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
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

func (r *FileURLRepository) Save(shortID, originalURL string) {
	r.mu.Lock()
	r.store[shortID] = originalURL
	r.mu.Unlock()

	if r.file == nil {
		return
	}

	entry := FileEntry{
		UUID:        fmt.Sprintf("%d", len(r.store)),
		ShortURL:    shortID,
		OriginalURL: originalURL,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Printf("Error marhaling to JSON: %v\n", err)
		return
	}

	_, err = r.file.Write(append(data, '\n'))
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
	}
}

func (r *FileURLRepository) loadFromFile() {
	f, err := os.Open(r.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Error reading file:: %v\n", err)
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
			fmt.Printf("Error unmarshaling line form file: %v\n", err)
			continue
		}

		if entry.ShortURL != "" && entry.OriginalURL != "" {
			r.store[entry.ShortURL] = entry.OriginalURL
		}
	}
}
