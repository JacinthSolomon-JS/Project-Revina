package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"project-revina/internal/engine"
)

// PlanList renders the ordered set of steps for review before anything runs.
// It is purely presentational: no state fields are written here.
type PlanList struct {
	Steps []engine.Step
}

var (
	idStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("45"))
	privTagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("208")).Padding(0, 1)
	descStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
)

func (l PlanList) Render() string {
	var b strings.Builder
	for i, s := range l.Steps {
		fmt.Fprintf(&b, "%3d. ", i+1)
		b.WriteString(idStyle.Render(s.ID()))
		if s.NeedsPrivilege() {
			b.WriteString(" ")
			b.WriteString(privTagStyle.Render("sudo"))
		}
		b.WriteString("  ")
		b.WriteString(descStyle.Render(s.Describe()))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
