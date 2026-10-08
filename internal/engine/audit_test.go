package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestJSONStoreAppendsRecords(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s := NewJSONStore(path)

	e1 := AuditEntry{Time: time.Now(), StepID: "a", State: "missing", Applied: true}
	e2 := AuditEntry{Time: time.Now().Add(time.Second), StepID: "b", State: "satisfied", Applied: false}

	for _, e := range []AuditEntry{e1, e2} {
		if err := s.Append(context.Background(), e); err != nil {
			t.Fatalf("Append returned error: %v", err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), data)
	}
	if !strings.Contains(lines[0], `"step_id":"a"`) {
		t.Errorf("first line missing step a: %s", lines[0])
	}
	if !strings.Contains(lines[1], `"step_id":"b"`) {
		t.Errorf("second line missing step b: %s", lines[1])
	}
}

func TestJSONStoreCreatesDirs(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "dirs", "state.json")
	s := NewJSONStore(path)
	if err := s.Append(context.Background(), AuditEntry{StepID: "a", State: "missing"}); err != nil {
		t.Fatalf("Append with nested dirs returned error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("state file not created: %v", err)
	}
}
