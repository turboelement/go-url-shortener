package audit

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// HTTPObserver sends audit events as HTTP POST requests.
type HTTPObserver struct {
	targetURL string
	client    *http.Client
}

// NewHTTPObserver creates an observer that POSTs audit events to a URL.
func NewHTTPObserver(targetURL string) *HTTPObserver {
	return &HTTPObserver{
		targetURL: targetURL,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify sends the audit event as an HTTP POST request to the target URL.
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

	// Drain response body so the TCP connection can be reused
	io.Copy(io.Discard, resp.Body)

	return nil
}
