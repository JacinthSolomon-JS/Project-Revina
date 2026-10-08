package translators

import (
	"context"
	"fmt"
	"strings"

	"project-revina/internal/engine"
)

// Choco implements engine.Translator for Chocolatey.
type Choco struct {
	mapped
	r engine.Runner
}

func NewChoco(r engine.Runner) (*Choco, error) {
	table, err := loadTable("choco")
	if err != nil {
		return nil, err
	}
	return &Choco{mapped: mapped{table: table}, r: r}, nil
}

func (c *Choco) Name() string { return "choco" }

// Installed uses an exact local-only search: "choco list --local-only --exact".
// Exit codes are not meaningful, so parse the "x packages installed" summary.
func (c *Choco) Installed(ctx context.Context, pkg string) (bool, error) {
	out, err := c.r.Run(ctx, "choco", "list", "--local-only", "--exact", c.resolve(pkg))
	if err != nil {
		if notFound(err) {
			return false, err
		}
		return false, nil
	}
	return !strings.HasPrefix(strings.TrimSpace(out), "0 packages"), nil
}

func (c *Choco) Version(ctx context.Context, pkg string) (string, error) {
	out, err := c.r.Run(ctx, "choco", "list", "--local-only", "--exact", c.resolve(pkg))
	if err != nil {
		if notFound(err) {
			return "", err
		}
		return "", nil
	}
	for _, line := range strings.Split(out, "\n") {
		if _, version, ok := splitListing(line); ok && version != "" {
			return version, nil
		}
	}
	return "", nil
}

func (c *Choco) Install(ctx context.Context, pkg string) error {
	out, err := c.r.Run(ctx, "choco", "install", c.resolve(pkg), "-y")
	if err != nil {
		if notFound(err) {
			return err
		}
		return fmt.Errorf("choco install %s: %w (output: %s)", pkg, err, strings.TrimSpace(out))
	}
	return nil
}

func (c *Choco) InstallCommand(pkg string) (string, []string, error) {
	return "choco", []string{"install", c.resolve(pkg), "-y"}, nil
}

// NeedsPrivilege is true because Chocolatey requires an elevated shell. We do
// NOT wrap it in sudo: elevation on Windows is handled separately, and the
// executor warns when the plan is running non-elevated.
func (c *Choco) NeedsPrivilege() bool { return true }
