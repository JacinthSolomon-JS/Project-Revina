package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditEntry is one line of the append-only audit log. It is a record of what
// the executor did, never a source of truth for idempotency: state comes from
// Step.Check() probing the real system.
type AuditEntry struct {
	Time       time.Time `json:"time"`
	StepID     string    `json:"step_id"`
	State      string    `json:"state"`
	Applied    bool      `json:"applied"`
	Error      string    `json:"error,omitempty"`
	DurationMS int64     `json:"duration_ms"`
}

// Store persists AuditEntry records. Executors record into it best-effort: a
// failing store must not abort a plan.
type Store interface {
	Append(ctx context.Context, entry AuditEntry) error
}

// JSONStore is an append-only, line-delimited JSON log (state.json). Appending
// never rewrites history, so concurrent runs do not clobber each other.
type JSONStore struct {
	path string
	mu   sync.Mutex
}

// NewJSONStore returns a Store writing to path.
func NewJSONStore(path string) *JSONStore {
	return &JSONStore{path: path}
}

// DefaultStatePath returns the default audit log location for the tool:
// <osconfig>/devsec/state.json (e.g. ~/.config/devsec/state.json on unix).
func DefaultStatePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}
	return filepath.Join(dir, "devsec", "state.json"), nil
}

func (s *JSONStore) Append(_ context.Context, entry AuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open state file: %w", err)
	}
	defer f.Close()

	b, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode audit entry: %w", err)
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("append audit entry: %w", err)
	}
	return nil
}
