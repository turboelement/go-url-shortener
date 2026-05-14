package audit

import (
	"encoding/json"
	"fmt"
	"os"
)

// FileObserver writes audit events to a file as JSON lines.
type FileObserver struct {
	filePath string
	file     *os.File
}

// NewFileObserver creates an observer that appends audit events to a file.
func NewFileObserver(filePath string) (*FileObserver, error) {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open audit file %s: %w", filePath, err)
	}

	return &FileObserver{
		filePath: filePath,
		file:     f,
	}, nil
}

// Notify writes the audit event as a JSON line to the file.
func (fo *FileObserver) Notify(event AuditEvent) error {
	if fo.file == nil {
		return fmt.Errorf("file not opened: %s", fo.filePath)
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	_, err = fo.file.Write(append(data, '\n'))
	if err != nil {
		return err
	}

	return nil
}

// Close closes the underlying file.
func (fo *FileObserver) Close() error {
	if fo.file != nil {
		return fo.file.Close()
	}
	return nil
}
