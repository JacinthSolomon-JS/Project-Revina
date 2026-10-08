package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// ExecutorOptions configures a plan executor.
type ExecutorOptions struct {
	// Logger receives progress and diagnostics. Defaults to slog.Default().
	Logger *slog.Logger
	// Store receives audit records. May be nil to skip recording.
	Store Store
	// FailFast stops the plan at the first failing step. Default is to
	// continue past errors and report all results.
	FailFast bool
	// OnResult, when set, is invoked with each step result as soon as the
	// step completes. It is used by the TUI to stream progress; it is a
	// hook only and must not duplicate Check/Apply logic.
	OnResult func(StepResult)
}

// StepResult reports the outcome of one step.
type StepResult struct {
	Step     Step
	Before   State // valid only when Check succeeded
	CheckErr error
	Applied  bool
	Err      error
}

// Executor runs a plan: for each step it Check()s real system state and only
// calls Apply() when the step is Missing or Drifted.
type Executor struct {
	log      *slog.Logger
	store    Store
	failFast bool
	onResult func(StepResult)
}

func NewExecutor(opts ExecutorOptions) *Executor {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Executor{log: logger, store: opts.Store, failFast: opts.FailFast, onResult: opts.OnResult}
}

func (e *Executor) Execute(ctx context.Context, steps []Step) []StepResult {
	results := make([]StepResult, 0, len(steps))

	privileged := 0
	for _, s := range steps {
		if s.NeedsPrivilege() {
			privileged++
		}
	}
	if privileged > 0 && !IsElevated() {
		e.log.Warn("plan contains steps that require elevation; running non-elevated",
			"steps", privileged)
	}

	for _, s := range steps {
		res := e.execStep(ctx, s)
		if e.onResult != nil {
			e.onResult(res)
		}
		results = append(results, res)
		if e.failFast && res.Err != nil {
			e.log.Error("aborting plan after failed step", "step", s.ID())
			break
		}
	}
	return results
}

func (e *Executor) execStep(ctx context.Context, s Step) StepResult {
	res := StepResult{Step: s}
	start := time.Now()

	entry := AuditEntry{StepID: s.ID()}
	finalize := func() {
		if entry.DurationMS == 0 {
			entry.DurationMS = time.Since(start).Milliseconds()
		}
		if e.store != nil {
			if recErr := e.store.Append(ctx, entry); recErr != nil {
				// Audit is best-effort: a failing store must not abort a plan.
				e.log.Warn("failed to record audit entry", "step", s.ID(), "err", recErr)
			}
		}
	}
	defer finalize()

	state, err := s.Check(ctx)
	if err != nil {
		res.CheckErr = err
		entry.State = "check-error"
		entry.Error = err.Error()
		e.log.Error("check failed; skipping step", "step", s.ID(), "err", err)
		return res
	}

	res.Before = state
	entry.State = state.String()

	switch state {
	case Satisfied:
		e.log.Info("satisfied", "step", s.ID())
	case Missing, Drifted:
		attemptStart := time.Now()
		if applyErr := s.Apply(ctx); applyErr != nil {
			res.Err = applyErr
			res.Applied = true
			entry.Applied = true
			entry.Error = applyErr.Error()
			entry.DurationMS = time.Since(attemptStart).Milliseconds()
			e.log.Error("apply failed", "step", s.ID(), "err", applyErr)
		} else {
			res.Applied = true
			entry.Applied = true
			entry.DurationMS = time.Since(attemptStart).Milliseconds()
			e.log.Info("applied", "step", s.ID())
		}
	default:
		res.CheckErr = fmt.Errorf("engine bug: invalid state %v from step %s", state, s.ID())
		entry.State = "check-error"
		entry.Error = res.CheckErr.Error()
		e.log.Error("invalid state", "step", s.ID(), "state", state)
	}
	return res
}
