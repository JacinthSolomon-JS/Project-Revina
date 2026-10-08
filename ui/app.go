package ui

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"project-revina/internal/engine"
	"project-revina/ui/components"
)

// Options carries everything the TUI needs to run a plan. The TUI never
// mutates the system itself: all Check/Apply work happens through an
// engine.Executor running inside a tea.Cmd goroutine.
type Options struct {
	Profile  string
	Steps    []engine.Step
	Store    engine.Store
	Logger   *slog.Logger
	FailFast bool
	Elevated bool
}

// Run starts the Bubbletea program: first a plan-review screen, then the run
// with a progress bar and streaming per-step log, then a summary. It returns
// an error when the program itself fails or when any step failed.
func Run(ctx context.Context, opts Options) error {
	m := newModel(ctx, opts)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	if m.failed > 0 {
		return fmt.Errorf("%d step(s) failed", m.failed)
	}
	return nil
}

type phase int

const (
	phasePlan phase = iota
	phaseRun
	phaseDone
)

var frameN int

type stepResultMsg struct{ res engine.StepResult }
type runDoneMsg struct{}

type model struct {
	ctx   context.Context
	opts  Options
	phase phase

	header    string
	planView  string
	progress  progress.Model
	logView   viewport.Model
	resultsCh chan engine.StepResult

	done, applied, satisfied, skipped, failed int
	logLines                                  []string
	termW, termH                              int
}

func newModel(ctx context.Context, opts Options) *model {
	pg := progress.New(progress.WithDefaultGradient())
	pg.ShowPercentage = true
	return &model{
		ctx:   ctx,
		opts:  opts,
		phase: phasePlan,
		header: components.Header{
			Profile:  opts.Profile,
			OS:       osLabel(),
			Count:    len(opts.Steps),
			Elevated: opts.Elevated,
		}.Render(),
		planView:  components.PlanList{Steps: opts.Steps}.Render(),
		progress:  pg,
		logView:   viewport.New(0, 1),
		resultsCh: make(chan engine.StepResult, len(opts.Steps)+1),
	}
}

func (m *model) Init() tea.Cmd { return nil }

// startRun launches the engine.Executor on a goroutine, streaming each step
// result into m.resultsCh (closed when the plan finishes), and returns the
// tea.Cmd that pops the first result.
func (m *model) startRun() tea.Cmd {
	go func() {
		defer close(m.resultsCh)
		store := m.opts.Store
		if store == nil {
			store = noopStore{}
		}
		logger := m.opts.Logger
		if logger == nil {
			logger = slog.Default()
		}
		ex := engine.NewExecutor(engine.ExecutorOptions{
			Logger:   logger,
			Store:    store,
			FailFast: m.opts.FailFast,
			OnResult: func(r engine.StepResult) {
				m.resultsCh <- r
			},
		})
		ex.Execute(m.ctx, m.opts.Steps)
	}()
	return m.waitResults()
}

func (m *model) waitResults() tea.Cmd {
	return func() tea.Msg {
		res, ok := <-m.resultsCh
		if !ok {
			return runDoneMsg{}
		}
		return stepResultMsg{res}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.applySize(msg.Width, msg.Height)
		return m, nil
	case progress.FrameMsg:
		frameN++
		next, cmd := m.progress.Update(msg)
		m.progress = next.(progress.Model)
		println("DBGF", frameN, cmd != nil, m.progress.IsAnimating(), strings.Count(m.View(), "\u2588"))
		return m, cmd
	}
	switch m.phase {
	case phasePlan:
		return m.updatePlan(msg)
	case phaseRun:
		return m.updateRun(msg)
	default: // phaseDone: show the summary until the user dismisses it
		if k, ok := msg.(tea.KeyMsg); ok {
			switch k.String() {
			case "q", "esc", "enter":
				return m, tea.Quit
			}
		}
		return m, nil
	}
}

// applySize resizes the run-view widgets. Heights and widths are clamped so a
// tiny or zero-sized terminal can never give the viewport a negative height
// (which used to panic visibleLines with a slice out of range).
//
// The log viewport is sized so the *entire* view fits the terminal: bubbletea
// only paints the last height lines, so a view taller than the terminal would
// push the progress bar off the top of the painted window and freeze it
// visually. The done view is taller (summary + hint), so it needs extra room.
func (m *model) applySize(width, height int) {
	m.termW, m.termH = width, height
	overhead := 4 // header, blanks, progress bar
	if m.phase == phaseDone {
		overhead = 7 // + summary + hint
	}
	m.logView.Width = max(0, width)
	m.logView.Height = max(1, height-overhead)
	m.progress.Width = max(10, width-4)
}

func (m *model) updatePlan(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, tea.Quit
		case "enter", " ":
			m.phase = phaseRun
			return m, m.startRun()
		}
	}
	return m, nil
}

func (m *model) updateRun(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case stepResultMsg:
		return m, tea.Batch(m.record(msg.res), m.waitResults())
	case runDoneMsg:
		m.phase = phaseDone
		m.applySize(m.termW, m.termH) // make room for summary + hint
		return m, nil
	}
	return m, nil
}

// record counts a finished step, appends its log line, and returns the
// progress bar's animation command (which must be run for the bar to move).
func (m *model) record(res engine.StepResult) tea.Cmd {
	m.done++
	frac := 0.0
	if n := len(m.opts.Steps); n > 0 {
		frac = float64(m.done) / float64(n)
	}
	cmd := m.progress.SetPercent(frac)

	style := lipgloss.NewStyle()
	var line string
	switch {
	case res.Err != nil:
		m.failed++
		style = style.Foreground(lipgloss.Color("9")).Bold(true)
		line = "FAILED   " + res.Step.ID() + ": " + res.Err.Error()
	case res.CheckErr != nil:
		m.skipped++
		style = style.Foreground(lipgloss.Color("11"))
		line = "SKIPPED  " + res.Step.ID() + ": " + res.CheckErr.Error()
	case res.Applied:
		m.applied++
		style = style.Foreground(lipgloss.Color("10"))
		line = "APPLIED  " + res.Step.ID()
	default:
		m.satisfied++
		style = style.Foreground(lipgloss.Color("6"))
		line = "OK       " + res.Step.ID() + " (already satisfied)"
	}
	m.logLines = append(m.logLines, style.Render(line))
	m.logView.SetContent(strings.Join(m.logLines, "\n"))
	m.logView.GotoBottom()
	return cmd
}

func (m *model) View() string {
	switch m.phase {
	case phasePlan:
		return lipgloss.NewStyle().Padding(1).Render(
			m.header + "\n\n" + m.planView + "\n\n" + hintStyle.Render("enter / space: start    q / esc: quit"))
	case phaseRun:
		return m.header + "\n\n" + m.progress.View() + "\n\n" + m.logView.View()
	default:
		return m.header + "\n\n" + m.progress.View() + "\n\n" + m.logView.View() +
			"\n\n" + summaryStyle.Render(m.summary()) + "\n" +
			hintStyle.Render("q / esc / enter: quit")
	}
}

func (m *model) summary() string {
	return fmt.Sprintf("%d steps: %d satisfied, %d applied, %d skipped, %d failed",
		m.done, m.satisfied, m.applied, m.skipped, m.failed)
}

var (
	hintStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	summaryStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
)

func osLabel() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	default:
		return runtime.GOOS
	}
}

type noopStore struct{}

func (noopStore) Append(_ context.Context, _ engine.AuditEntry) error { return nil }

var _ engine.Store = noopStore{}
