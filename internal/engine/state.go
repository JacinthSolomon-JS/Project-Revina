package engine

import "fmt"

// State describes whether a Step is already in the desired state on the host.
type State int

const (
	// Satisfied means the step's goal is already met; Apply is skipped.
	Satisfied State = iota
	// Missing means the step's goal is not met; Apply is warranted.
	Missing
	// Drifted means the step's goal was met previously but has changed; Apply is
	// warranted to restore the desired state.
	Drifted
)

func (s State) String() string {
	switch s {
	case Satisfied:
		return "satisfied"
	case Missing:
		return "missing"
	case Drifted:
		return "drifted"
	default:
		return fmt.Sprintf("State(%d)", int(s))
	}
}
