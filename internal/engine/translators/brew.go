package translators

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"project-revina/internal/engine"
	"project-revina/internal/engine/mapping"
)

// Brew implements engine.Translator for Homebrew on macOS.
type Brew struct {
	r     engine.Runner
	table *mapping.Table
}

// NewBrew builds a Homebrew translator on the given Runner. It keeps this
// package a leaf: the returned value satisfies engine.Translator.
func NewBrew(r engine.Runner) (*Brew, error) {
	table, err := mapping.Load("brew")
	if err != nil {
		return nil, err
	}
	return &Brew{r: r, table: table}, nil
}

func (b *Brew) Name() string { return "brew" }

// formula resolves a logical package name to its Homebrew formula name.
func (b *Brew) formula(pkg string) string { return b.table.Resolve(pkg) }

// Installed reports whether a formula is installed. A missing brew binary is a
// real error; an empty/unlisted formula is treated as not installed.
func (b *Brew) Installed(ctx context.Context, pkg string) (bool, error) {
	out, err := b.r.Run(ctx, "brew", "list", "--versions", b.formula(pkg))
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return false, fmt.Errorf("brew: %w", err)
		}
		return false, nil
	}
	return strings.TrimSpace(out) != "", nil
}

func (b *Brew) Install(ctx context.Context, pkg string) error {
	out, err := b.r.Run(ctx, "brew", "install", b.formula(pkg))
	if err != nil {
		return fmt.Errorf("brew install %s: %w (output: %s)", b.formula(pkg), err, strings.TrimSpace(out))
	}
	return nil
}

func (b *Brew) Version(ctx context.Context, pkg string) (string, error) {
	out, err := b.r.Run(ctx, "brew", "list", "--versions", b.formula(pkg))
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("brew: %w", err)
		}
		return "", nil
	}
	fields := strings.Fields(out)
	if len(fields) < 2 {
		return "", nil
	}
	return strings.Join(fields[1:], " "), nil
}

func (b *Brew) InstallCommand(pkg string) (string, []string, error) {
	return "brew", []string{"install", b.formula(pkg)}, nil
}

func (b *Brew) NeedsPrivilege() bool { return false }
