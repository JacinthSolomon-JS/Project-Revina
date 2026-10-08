package ui

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"project-revina/internal/engine"
)

// stubStep is a Step whose Check/Apply never touch a real system.
type stubStep struct {
	id     string
	state  engine.State
	apply  error
	priv   bool
	checks int
}

func (s *stubStep) ID() string           { return s.id }
func (s *stubStep) Describe() string     { return "stub " + s.id }
func (s *stubStep) NeedsPrivilege() bool { return s.priv }
func (s *stubStep) Check(context.Context) (engine.State, error) {
	s.checks++
	return s.state, nil
}
func (s *stubStep) Apply(context.Context) error { return s.apply }

func TestModelPlanThenRun(t *testing.T) {
	steps := []engine.Step{
		&stubStep{id: "a", state: engine.Satisfied},
		&stubStep{id: "b", state: engine.Missing},
	}
	m := newModel(context.Background(), Options{Profile: "dev", Steps: steps})

	if m.phase != phasePlan {
		t.Fatalf("phase = %v, want plan", m.phase)
	}
	if m.planView == "" {
		t.Error("plan view is empty")
	}

	// Start the run from the plan screen.
	sm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = sm.(*model)
	if m.phase != phaseRun {
		t.Fatalf("phase after enter = %v, want run", m.phase)
	}

	// Drain the two step results then the done signal.
	for i := 0; i < len(steps); i++ {
		got := cmd()
		_, ok := got.(stepResultMsg)
		if !ok {
			t.Fatalf("cmd %d returned %T, want stepResultMsg", i, got)
		}
		sm, cmd = m.Update(got)
		m = sm.(*model)
	}
	done := cmd()
	if _, ok := done.(runDoneMsg); !ok {
		t.Fatalf("final cmd returned %T, want runDoneMsg", done)
	}
	sm, finalCmd := m.Update(done)
	m = sm.(*model)
	if m.phase != phaseDone {
		t.Fatalf("phase = %v, want done", m.phase)
	}
	_ = finalCmd
}

func TestModelRecordsFailures(t *testing.T) {
	steps := []engine.Step{
		&stubStep{id: "ok", state: engine.Satisfied},
		&stubStep{id: "boom", state: engine.Drifted, apply: errors.New("nope")},
		&stubStep{id: "chkerr", state: engine.Missing},
	}
	m := newModel(context.Background(), Options{Steps: steps})
	sm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = sm.(*model)

	got := cmd()
	res := got.(stepResultMsg)
	m.record(res.res) // ok, satisfied

	got = cmd()
	m.record(got.(stepResultMsg).res) // boom, failed

	if m.failed != 1 {
		t.Errorf("failed = %d, want 1", m.failed)
	}
	if m.satisfied != 1 {
		t.Errorf("satisfied = %d, want 1", m.satisfied)
	}
	_ = m
}

func TestModelQuitFromPlan(t *testing.T) {
	m := newModel(context.Background(), Options{Steps: []engine.Step{&stubStep{id: "a", state: engine.Satisfied}}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	expectQuit(t, cmd)
}

func TestModelQuitFromRun(t *testing.T) {
	m := newModel(context.Background(), Options{Steps: []engine.Step{&stubStep{id: "a", state: engine.Satisfied}}})
	m.phase = phaseRun
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	// During run, q is ignored (run continues); only the run loop quits.
	if cmd != nil {
		t.Errorf("unexpected cmd after q during run: %v", cmd)
	}
}

func expectQuit(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a quit cmd")
	}
	msg := cmd()
	if msg != tea.Quit() {
		t.Errorf("msg = %T, want tea.QuitMsg", msg)
	}
}

func TestEngineStoreNoop(t *testing.T) {
	var s engine.Store = noopStore{}
	if err := s.Append(context.Background(), engine.AuditEntry{StepID: "x"}); err != nil {
		t.Errorf("noop store append: %v", err)
	}
}
