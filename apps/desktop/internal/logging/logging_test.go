package logging

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAddsTimestampAndLevel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "connections.jsonl")
	sink := NewSink(path)
	sink.Write("log", map[string]any{"level": "warning", "message": "hello"})
	sink.Write("reconnecting", map[string]any{"seconds": 2})

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var lines []map[string]any
	for scanner.Scan() {
		var line map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, line)
	}
	if len(lines) != 2 {
		t.Fatalf("%d lines", len(lines))
	}
	if lines[0]["event"] != "log" || lines[0]["timestamp"] == "" {
		t.Fatalf("first line %v", lines[0])
	}
	if lines[1]["level"] != "warning" {
		t.Fatalf("derived level %v", lines[1])
	}
}

func TestNilSinkAndRotation(t *testing.T) {
	var nilSink *Sink
	nilSink.Write("log", nil) // must not panic

	dir := t.TempDir()
	path := filepath.Join(dir, "connections.jsonl")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", maxBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	NewSink(path).Write("log", nil)
	if _, err := os.Stat(path + ".previous"); err != nil {
		t.Fatalf("rotated file: %v", err)
	}
}
