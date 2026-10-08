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
		logView:   viewport.New(0, 0),
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
	switch m.phase {
	case phasePlan:
		return m.updatePlan(msg)
	case phaseRun:
		return m.updateRun(msg)
	default:
		return m, nil
	}
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
	case tea.WindowSizeMsg:
		m.logView.Width = msg.Width
		m.logView.Height = msg.Height - 4
		m.progress.Width = msg.Width - 4
	}
	return m, nil
}

func (m *model) updateRun(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case stepResultMsg:
		m.record(msg.res)
		return m, m.waitResults()
	case runDoneMsg:
		m.phase = phaseDone
		return m, tea.Quit
	case tea.WindowSizeMsg:
		m.logView.Width = msg.Width
		m.logView.Height = msg.Height - 4
		m.progress.Width = msg.Width - 4
	}
	return m, nil
}

func (m *model) record(res engine.StepResult) {
	m.done++
	m.progress.SetPercent(float64(m.done) / float64(len(m.opts.Steps)))

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
			"\n\n" + summaryStyle.Render(m.summary())
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
