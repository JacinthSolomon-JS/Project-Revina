package engine

import (
	"fmt"
	"io"
)

// PrintPlan writes a human-readable plan for --dry-run. It shows the exact
// commands each Apply would run, never executes them.
func PrintPlan(w io.Writer, steps []Step) error {
	privileged := 0
	for _, s := range steps {
		if s.NeedsPrivilege() {
			privileged++
		}
	}

	if _, err := fmt.Fprintf(w, "Plan: %d step(s), %d requiring elevation (--dry-run: no changes made)\n", len(steps), privileged); err != nil {
		return err
	}
	for i, s := range steps {
		if _, err := fmt.Fprintf(w, "  %3d. %s\n", i+1, s.Describe()); err != nil {
			return err
		}
	}
	return nil
}
