package engine

import (
	"context"
	"fmt"
	"strings"
)

// DetectLinuxPM finds the host distro's package manager via the Runner so it
// stays unit-testable. Prefers apt, then dnf, then pacman.
func DetectLinuxPM(ctx context.Context, r Runner) (string, error) {
	out, err := r.Run(ctx, "sh", "-c", "if command -v apt-get >/dev/null 2>&1; then echo apt; elif command -v dnf >/dev/null 2>&1; then echo dnf; elif command -v pacman >/dev/null 2>&1; then echo pacman; fi")
	if err != nil {
		return "", fmt.Errorf("detect linux package manager: %w", err)
	}
	pm := strings.TrimSpace(out)
	if pm == "" {
		return "", fmt.Errorf("no supported package manager found (apt/dnf/pacman)")
	}
	return pm, nil
}

// DetectWindowsPM finds the host package manager via the Runner. Prefers
// winget, then scoop, then choco.
func DetectWindowsPM(ctx context.Context, r Runner) (string, error) {
	for _, cand := range []string{"winget", "scoop", "choco"} {
		if _, err := r.Run(ctx, "where", cand); err == nil {
			return cand, nil
		}
	}
	return "", fmt.Errorf("no supported package manager found (winget/scoop/choco)")
}
