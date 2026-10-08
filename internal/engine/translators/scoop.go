package translators

import (
	"context"
	"fmt"
	"strings"

	"project-revina/internal/engine"
)

// Scoop implements engine.Translator for Windows Scoop.
type Scoop struct {
	mapped
	r engine.Runner
}

func NewScoop(r engine.Runner) (*Scoop, error) {
	table, err := loadTable("scoop")
	if err != nil {
		return nil, err
	}
	return &Scoop{mapped: mapped{table: table}, r: r}, nil
}

func (s *Scoop) Name() string { return "scoop" }

// Installed filters "scoop list" output for the app name.
func (s *Scoop) Installed(ctx context.Context, pkg string) (bool, error) {
	out, err := s.r.Run(ctx, "scoop", "list", s.resolve(pkg))
	if err != nil {
		if notFound(err) {
			return false, err
		}
		return false, nil
	}
	_, _, ok := splitListing(out)
	return ok, nil
}

func (s *Scoop) Version(ctx context.Context, pkg string) (string, error) {
	out, err := s.r.Run(ctx, "scoop", "list", s.resolve(pkg))
	if err != nil {
		if notFound(err) {
			return "", err
		}
		return "", nil
	}
	_, version, ok := splitListing(out)
	if !ok {
		return "", nil
	}
	return version, nil
}

func (s *Scoop) Install(ctx context.Context, pkg string) error {
	out, err := s.r.Run(ctx, "scoop", "install", s.resolve(pkg))
	if err != nil {
		if notFound(err) {
			return err
		}
		return fmt.Errorf("scoop install %s: %w (output: %s)", pkg, err, strings.TrimSpace(out))
	}
	return nil
}

func (s *Scoop) InstallCommand(pkg string) (string, []string, error) {
	return "scoop", []string{"install", s.resolve(pkg)}, nil
}

func (s *Scoop) NeedsPrivilege() bool { return false }
