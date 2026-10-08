package engine

import "context"

// Step is the unit of work. Every task in the system, from installing a package
// to a hardening change, is expressed as a Step. Do not bypass this interface.
type Step interface {
	// ID is a stable, unique identifier for this step.
	ID() string
	// Check inspects the real system and reports whether the step's goal is
	// already met. It MUST NOT mutate anything.
	Check(ctx context.Context) (State, error)
	// Apply brings the system to the step's goal state. It MUST be safe to
	// re-run (idempotent). The executor only calls it when Check returns
	// Missing or Drifted.
	Apply(ctx context.Context) error
	// Describe returns a human-readable description that shows the exact
	// command(s) or config changes Apply would perform. Used by dry-run and UI.
	Describe() string
	// NeedsPrivilege reports whether Apply requires elevation (sudo / admin).
	NeedsPrivilege() bool
}
