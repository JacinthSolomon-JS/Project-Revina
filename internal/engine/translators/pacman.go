package translators

import (
	"context"
	"fmt"
	"strings"

	"project-revina/internal/engine"
)

// Pacman implements engine.Translator for Arch Linux.
type Pacman struct {
	mapped
	r engine.Runner
}

func NewPacman(r engine.Runner) (*Pacman, error) {
	table, err := loadTable("pacman")
	if err != nil {
		return nil, err
	}
	return &Pacman{mapped: mapped{table: table}, r: r}, nil
}

func (p *Pacman) Name() string { return "pacman" }

func (p *Pacman) Installed(ctx context.Context, pkg string) (bool, error) {
	_, err := p.r.Run(ctx, "pacman", "-Q", p.resolve(pkg))
	if err != nil {
		if notFound(err) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

func (p *Pacman) Version(ctx context.Context, pkg string) (string, error) {
	out, err := p.r.Run(ctx, "pacman", "-Q", p.resolve(pkg))
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

func (p *Pacman) Install(ctx context.Context, pkg string) error {
	name, args := elevate("pacman", []string{"-S", "--noconfirm", p.resolve(pkg)})
	out, err := p.r.Run(ctx, name, args...)
	if err != nil {
		if notFound(err) {
			return err
		}
		return fmt.Errorf("pacman -S %s: %w (output: %s)", pkg, err, strings.TrimSpace(out))
	}
	return nil
}

func (p *Pacman) InstallCommand(pkg string) (string, []string, error) {
	name, args := elevate("pacman", []string{"-S", "--noconfirm", p.resolve(pkg)})
	return name, args, nil
}

func (p *Pacman) NeedsPrivilege() bool { return true }
