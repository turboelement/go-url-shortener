package audit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

type HTTPObserver struct {
	targetURL string
	client    *http.Client
}

func NewHTTPObserver(targetURL string) *HTTPObserver {
	return &HTTPObserver{
		targetURL: targetURL,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (ho *HTTPObserver) Notify(event AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	resp, err := ho.client.Post(ho.targetURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
