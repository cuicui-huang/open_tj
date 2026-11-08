// Package logging writes the JSONL diagnostic log the app keeps beside the
// connection log of earlier versions.
package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// maxBytes is the size after which the log rotates to .previous.
const maxBytes = 2 * 1024 * 1024

// Sink appends one JSON object per line to a file. A nil Sink is valid and
// discards everything, so callers do not need to check.
type Sink struct {
	mu   sync.Mutex
	path string
}

// NewSink returns a sink writing to path. The directory is created on the
// first write.
func NewSink(path string) *Sink {
	return &Sink{path: path}
}

// Write appends an event. The timestamp is added unless fields has one.
func (s *Sink) Write(event string, fields map[string]any) {
	if s == nil || s.path == "" {
		return
	}
	if fields == nil {
		fields = map[string]any{}
	}
	if _, ok := fields["timestamp"]; !ok {
		fields["timestamp"] = time.Now().UTC().Format(time.RFC3339Nano)
	}
	fields["event"] = event
	if _, ok := fields["level"]; !ok {
		fields["level"] = levelFor(event)
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return
	}
	data = append(data, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	if info, err := os.Stat(s.path); err == nil && info.Size() > maxBytes {
		_ = os.Rename(s.path, s.path+".previous")
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = f.Write(data)
	_ = f.Close()
}

func levelFor(event string) string {
	switch event {
	case "reconnecting":
		return "warning"
	case "failed":
		return "error"
	default:
		return "info"
	}
}
