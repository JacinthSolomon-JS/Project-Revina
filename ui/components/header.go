package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Header is the fixed top bar shown by the TUI on every screen.
type Header struct {
	Profile string
	OS      string
	Count   int
	// Elevated reports whether the process already has admin rights; only
	// cosmetic, never changes behaviour in the UI layer.
	Elevated bool
}

var headerStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("229")).
	Background(lipgloss.Color("61")).
	Padding(0, 1)

var tagStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("229")).
	Background(lipgloss.Color("240")).
	Padding(0, 1)

func (h Header) render() string {
	parts := []string{headerStyle.Render(" devsec "), tagStyle.Render("profile: " + h.Profile),
		tagStyle.Render(h.OS), tagStyle.Render("steps: " + itoa(h.Count))}
	if h.Elevated {
		parts = append(parts, tagStyle.Render("elevated"))
	}
	return strings.Join(parts, " ")
}

// Render returns the header line.
func (h Header) Render() string { return h.render() }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
