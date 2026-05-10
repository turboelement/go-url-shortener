package audit

import (
	"encoding/json"
	"fmt"
	"os"
)

type FileObserver struct {
	filePath string
	file     *os.File
}

func NewFileObserver(filePath string) *FileObserver {
	fo := &FileObserver{
		filePath: filePath,
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error opening file: %s: %v\n", filePath, err)
	} else {
		fo.file = f
	}

	return fo
}

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
