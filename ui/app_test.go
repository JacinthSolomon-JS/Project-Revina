package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/progress"
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
		got := nextMsg(t, cmd)
		_, ok := got.(stepResultMsg)
		if !ok {
			t.Fatalf("cmd %d returned %T, want stepResultMsg", i, got)
		}
		sm, cmd = m.Update(got)
		m = sm.(*model)
	}
	done := nextMsg(t, cmd)
	if _, ok := done.(runDoneMsg); !ok {
		t.Fatalf("final cmd returned %T, want runDoneMsg", done)
	}
	sm, finalCmd := m.Update(done)
	m = sm.(*model)
	if m.phase != phaseDone {
		t.Fatalf("phase = %v, want done", m.phase)
	}
	if finalCmd != nil {
		t.Error("reaching done must not quit: the summary stays until dismissed")
	}

	// The summary screen waits for a key before quitting.
	sm, qCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = sm.(*model)
	expectQuit(t, qCmd)
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

// nextMsg runs a cmd, unwrapping tea.BatchMsg and skipping progress-bar
// animation frame ticks, and returns the first meaningful message.
func nextMsg(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a cmd")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return msg
	}
	var first tea.Msg
	for _, c := range batch {
		if c == nil {
			continue
		}
		got := c()
		if got == nil {
			continue
		}
		if _, isFrame := got.(progress.FrameMsg); isFrame {
			continue
		}
		if first == nil {
			first = got
		}
		switch got.(type) {
		case stepResultMsg, runDoneMsg:
			return got
		}
	}
	if first == nil {
		t.Fatal("cmd produced no message")
	}
	return first
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

// Regression: a zero-sized (or tiny) terminal used to give the log viewport a
// negative height, panicking visibleLines with "slice bounds out of range"
// when the first result arrived.
func TestRecordAtZeroSizedTerminalDoesNotPanic(t *testing.T) {
	steps := []engine.Step{&stubStep{id: "a", state: engine.Satisfied}}
	m := newModel(context.Background(), Options{Steps: steps})

	sm, _ := m.Update(tea.WindowSizeMsg{Width: 0, Height: 0})
	m = sm.(*model)
	sm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = sm.(*model)

	res, ok := cmd().(stepResultMsg)
	if !ok {
		t.Fatal("expected first step result")
	}
	frame := m.record(res.res)
	if frame == nil {
		t.Fatal("record must return the progress animation command")
	}
	if m.logView.Height < 1 {
		t.Errorf("log viewport height = %d, want >= 1", m.logView.Height)
	}
}

func TestRecordAdvancesProgress(t *testing.T) {
	steps := []engine.Step{
		&stubStep{id: "a", state: engine.Satisfied},
		&stubStep{id: "b", state: engine.Satisfied},
	}
	m := newModel(context.Background(), Options{Steps: steps})

	m.record(engine.StepResult{Step: steps[0]})
	if got := m.progress.Percent(); got != 0.5 {
		t.Errorf("progress target = %v, want 0.5", got)
	}
	m.record(engine.StepResult{Step: steps[1]})
	if got := m.progress.Percent(); got != 1 {
		t.Errorf("progress target = %v, want 1", got)
	}
}

func TestApplySizeClampsTinyTerminals(t *testing.T) {
	m := newModel(context.Background(), Options{})
	for _, size := range [][2]int{{0, 0}, {80, 4}, {2, 2}} {
		m.applySize(size[0], size[1])
		if m.logView.Height < 1 {
			t.Errorf("applySize(%d,%d): log height = %d, want >= 1", size[0], size[1], m.logView.Height)
		}
		if m.progress.Width < 10 {
			t.Errorf("applySize(%d,%d): progress width = %d, want >= 10", size[0], size[1], m.progress.Width)
		}
	}
}

// Regression: the done view used to be taller than the terminal, so bubbletea
// dropped the top lines (including the progress bar) from the painted window
// and the bar froze visually at 0% forever.
func TestDoneViewFitsTerminalHeight(t *testing.T) {
	const height = 24
	steps := []engine.Step{
		&stubStep{id: "a", state: engine.Satisfied},
		&stubStep{id: "b", state: engine.Satisfied},
	}
	m := newModel(context.Background(), Options{Profile: "dev", Steps: steps})
	sm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: height})
	m = sm.(*model)
	sm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = sm.(*model)
	for i := 0; i <= len(steps); i++ {
		got := nextMsg(t, cmd)
		sm, cmd = m.Update(got)
		m = sm.(*model)
		if m.phase == phaseDone {
			break
		}
	}
	if m.phase != phaseDone {
		t.Fatalf("phase = %v, want done", m.phase)
	}

	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) > height {
		t.Fatalf("done view is %d lines for a %d-row terminal:\n%s", len(lines), height, view)
	}
	// bubbletea paints only the last `height` lines; the bar must be inside.
	painted := lines[max(0, len(lines)-height):]
	if !strings.Contains(strings.Join(painted, "\n"), "%") {
		t.Errorf("progress bar missing from the painted window:\n%s", view)
	}
}
