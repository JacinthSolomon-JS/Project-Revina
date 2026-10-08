package engine_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"

	"project-revina/internal/engine"
	"project-revina/internal/engine/translators"
)

func newBrewStep(t *testing.T, fr *engine.FakeRunner) engine.Step {
	t.Helper()
	tr, err := translators.NewBrew(fr)
	if err != nil {
		t.Fatalf("NewBrew returned error: %v", err)
	}
	return engine.NewPackageStep("dev:ripgrep", "ripgrep", tr, "")
}

func TestExecutorSkipsSatisfied(t *testing.T) {
	t.Parallel()

	fr := engine.NewFakeRunner(map[string]engine.FakeResponse{
		"brew list --versions ripgrep": {Output: "ripgrep 14.1.0"},
	})
	ex := engine.NewExecutor(engine.ExecutorOptions{Logger: discardLogger(t)})
	results := ex.Execute(context.Background(), []engine.Step{newBrewStep(t, fr)})

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	r := results[0]
	if r.Applied {
		t.Error("Applied = true, want false for satisfied step")
	}
	if r.Before != engine.Satisfied {
		t.Errorf("Before = %v, want Satisfied", r.Before)
	}
	if r.Err != nil || r.CheckErr != nil {
		t.Errorf("unexpected errors: %v / %v", r.Err, r.CheckErr)
	}
	if got := len(fr.Calls()); got != 1 {
		t.Errorf("runner calls = %d, want 1 (check only, no apply)", got)
	}
}

func TestExecutorAppliesMissing(t *testing.T) {
	t.Parallel()

	fr := engine.NewFakeRunner(map[string]engine.FakeResponse{
		"brew list --versions ripgrep": {Err: errors.New("not installed")},
		"brew install ripgrep":         {},
	})
	ex := engine.NewExecutor(engine.ExecutorOptions{Logger: discardLogger(t)})
	results := ex.Execute(context.Background(), []engine.Step{newBrewStep(t, fr)})

	r := results[0]
	if !r.Applied {
		t.Error("Applied = false, want true for missing step")
	}
	if r.Before != engine.Missing {
		t.Errorf("Before = %v, want Missing", r.Before)
	}
	calls := fr.Calls()
	if len(calls) != 2 {
		t.Fatalf("runner calls = %d, want 2 (check + apply)", len(calls))
	}
	if calls[1].Args[0] != "install" {
		t.Errorf("second call args = %v, want install", calls[1].Args)
	}
}

func TestExecutorRecordsAudit(t *testing.T) {
	t.Parallel()

	fr := engine.NewFakeRunner(map[string]engine.FakeResponse{
		"brew list --versions ripgrep": {Err: errors.New("not installed")},
		"brew install ripgrep":         {},
	})
	store := &recordingStore{}
	ex := engine.NewExecutor(engine.ExecutorOptions{Logger: discardLogger(t), Store: store})
	ex.Execute(context.Background(), []engine.Step{newBrewStep(t, fr)})

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(store.entries))
	}
	e := store.entries[0]
	if e.StepID != "dev:ripgrep" || !e.Applied || e.State != "missing" {
		t.Errorf("unexpected audit entry: %+v", e)
	}
}

func TestExecutorContinuesOnErrorByDefault(t *testing.T) {
	t.Parallel()

	fr := engine.NewFakeRunner(map[string]engine.FakeResponse{
		"brew list --versions a": {Err: errors.New("no")},
		"brew install a":         {Err: errors.New("boom")},
		"brew list --versions b": {Err: errors.New("no")},
		"brew install b":         {},
	})
	tr, err := translators.NewBrew(fr)
	if err != nil {
		t.Fatalf("NewBrew returned error: %v", err)
	}
	steps := []engine.Step{
		engine.NewPackageStep("t:a", "a", tr, ""),
		engine.NewPackageStep("t:b", "b", tr, ""),
	}
	ex := engine.NewExecutor(engine.ExecutorOptions{Logger: discardLogger(t)})
	results := ex.Execute(context.Background(), steps)

	if len(results) != 2 {
		t.Fatalf("results = %d, want 2 (default continues)", len(results))
	}
	if results[0].Err == nil {
		t.Error("first step Err = nil, want apply error")
	}
	if !results[1].Applied {
		t.Error("second step not applied, want applied (continue on error)")
	}
}

func TestExecutorFailFast(t *testing.T) {
	t.Parallel()

	fr := engine.NewFakeRunner(map[string]engine.FakeResponse{
		"brew list --versions a": {Err: errors.New("no")},
		"brew install a":         {Err: errors.New("boom")},
		"brew list --versions b": {Err: errors.New("no")},
		"brew install b":         {},
	})
	tr, err := translators.NewBrew(fr)
	if err != nil {
		t.Fatalf("NewBrew returned error: %v", err)
	}
	steps := []engine.Step{
		engine.NewPackageStep("t:a", "a", tr, ""),
		engine.NewPackageStep("t:b", "b", tr, ""),
	}
	ex := engine.NewExecutor(engine.ExecutorOptions{Logger: discardLogger(t), FailFast: true})
	results := ex.Execute(context.Background(), steps)

	if len(results) != 1 {
		t.Errorf("results = %d, want 1 (fail fast)", len(results))
	}
}

type recordingStore struct {
	mu      sync.Mutex
	entries []engine.AuditEntry
}

func (s *recordingStore) Append(_ context.Context, e engine.AuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, e)
	return nil
}

func discardLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}
