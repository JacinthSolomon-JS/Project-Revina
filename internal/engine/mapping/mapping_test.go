package mapping

import (
	"strings"
	"testing"
)

func TestLoadAndResolve(t *testing.T) {
	t.Parallel()

	tab, err := Load("brew")
	if err != nil {
		t.Fatalf("Load(brew) returned error: %v", err)
	}
	if tab.Manager() != "brew" {
		t.Errorf("Manager() = %q, want brew", tab.Manager())
	}

	cases := map[string]string{
		"rg":      "ripgrep", // mapped
		"exa":     "eza",     // mapped
		"ripgrep": "ripgrep", // identity
		" fzf ":   "fzf",     // trimmed identity
		"nothere": "nothere", // unknown -> identity
	}
	for logical, want := range cases {
		if got := tab.Resolve(logical); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", logical, got, want)
		}
	}
}

func TestLoadMissingTable(t *testing.T) {
	t.Parallel()

	if _, err := Load("nope"); err == nil {
		t.Fatal("Load(nope) returned nil error, want error")
	}
}

func TestResolveIgnoresEmptyMapping(t *testing.T) {
	t.Parallel()

	tab, err := Load("brew")
	if err != nil {
		t.Fatalf("Load(brew) returned error: %v", err)
	}
	if got := tab.Resolve(""); got != "" {
		t.Errorf("Resolve(\"\") = %q, want empty", got)
	}
	if got := tab.Resolve(strings.Repeat(" ", 3)); got != "" {
		t.Errorf("Resolve(whitespace) = %q, want empty", got)
	}
}
