package translators

import (
	"context"
	"fmt"
	"strings"

	"project-revina/internal/engine"
)

// Winget implements engine.Translator for Windows winget.
type Winget struct {
	mapped
	r engine.Runner
}

func NewWinget(r engine.Runner) (*Winget, error) {
	table, err := loadTable("winget")
	if err != nil {
		return nil, err
	}
	return &Winget{mapped: mapped{table: table}, r: r}, nil
}

func (w *Winget) Name() string { return "winget" }

// winget list exits 0 when the package is installed, 1 when not found.
func (w *Winget) Installed(ctx context.Context, pkg string) (bool, error) {
	_, err := w.r.Run(ctx, "winget", "list", "--id", w.resolve(pkg))
	if err != nil {
		if notFound(err) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

// Version is not reliably parseable from winget list output without JSON; keep
// returning ("", nil) until a caller needs it.
func (w *Winget) Version(context.Context, string) (string, error) { return "", nil }

func (w *Winget) Install(ctx context.Context, pkg string) error {
	args := []string{"install", "--id", w.resolve(pkg), "--silent",
		"--accept-package-agreements", "--accept-source-agreements"}
	out, err := w.r.Run(ctx, "winget", args...)
	if err != nil {
		if notFound(err) {
			return err
		}
		return fmt.Errorf("winget install %s: %w (output: %s)", pkg, err, strings.TrimSpace(out))
	}
	return nil
}

func (w *Winget) InstallCommand(pkg string) (string, []string, error) {
	return "winget", []string{"install", "--id", w.resolve(pkg), "--silent",
		"--accept-package-agreements", "--accept-source-agreements"}, nil
}

func (w *Winget) NeedsPrivilege() bool { return false }
