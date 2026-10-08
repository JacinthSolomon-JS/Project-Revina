package translators

import (
	"context"
	"strings"
	"testing"

	"project-revina/internal/engine"
)

func TestBrewResolvesLogicalNames(t *testing.T) {
	t.Parallel()

	fr := engine.NewFakeRunner(map[string]engine.FakeResponse{
		"brew list --versions ripgrep": {Output: "ripgrep 14.1.0"},
	})
	b, err := NewBrew(fr)
	if err != nil {
		t.Fatalf("NewBrew returned error: %v", err)
	}

	installed, err := b.Installed(context.Background(), "rg")
	if err != nil {
		t.Fatalf("Installed returned error: %v", err)
	}
	if !installed {
		t.Error("Installed(rg) = false, want true (rg maps to ripgrep)")
	}

	cmd, args, err := b.InstallCommand("exa")
	if err != nil {
		t.Fatalf("InstallCommand returned error: %v", err)
	}
	if strings.Join(append([]string{cmd}, args...), " ") != "brew install eza" {
		t.Errorf("InstallCommand(exa) = %q, want \"brew install eza\"", strings.Join(append([]string{cmd}, args...), " "))
	}
}
