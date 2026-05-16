package audit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubject_NoObservers_NoPanic(t *testing.T) {
	s := NewSubject()
	s.NotifyAll(AuditEvent{Timestamp: 1, Action: ActionShorten, URL: "http://test.com"})
	s.Close()
}

func TestSubject_WithFileObserver(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	s := NewSubject()
	fo, err := NewFileObserver(path)
	require.NoError(t, err)
	defer fo.Close()
	s.Register(fo)

	event := AuditEvent{
		Timestamp: 12345,
		Action:    ActionShorten,
		UserID:    "user",
		URL:       "https://example.com",
	}

	s.NotifyAll(event)
	s.Flush()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var decoded AuditEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, event, decoded)
	s.Close()
}

func TestSubject_WithHTTPObserver(t *testing.T) {
	var got AuditEvent
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	ho := NewHTTPObserver(ts.URL)

	event := AuditEvent{
		Timestamp: 12345,
		Action:    ActionShorten,
		URL:       "http://example.com",
	}
	err := ho.Notify(event)
	require.NoError(t, err)
	assert.Equal(t, event, got)
}
