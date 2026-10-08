package translators

import (
	"context"
	"fmt"
	"strings"

	"project-revina/internal/engine"
)

// Dnf implements engine.Translator for Fedora/RHEL.
type Dnf struct {
	mapped
	r engine.Runner
}

func NewDnf(r engine.Runner) (*Dnf, error) {
	table, err := loadTable("dnf")
	if err != nil {
		return nil, err
	}
	return &Dnf{mapped: mapped{table: table}, r: r}, nil
}

func (d *Dnf) Name() string { return "dnf" }

func (d *Dnf) Installed(ctx context.Context, pkg string) (bool, error) {
	_, err := d.r.Run(ctx, "rpm", "-q", d.resolve(pkg))
	if err != nil {
		if notFound(err) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

func (d *Dnf) Version(ctx context.Context, pkg string) (string, error) {
	out, err := d.r.Run(ctx, "rpm", "-q", "--queryformat", "%{VERSION}", d.resolve(pkg))
	if err != nil {
		if notFound(err) {
			return "", err
		}
		return "", nil
	}
	return strings.TrimSpace(out), nil
}

func (d *Dnf) Install(ctx context.Context, pkg string) error {
	name, args := elevate("dnf", []string{"install", "-y", d.resolve(pkg)})
	out, err := d.r.Run(ctx, name, args...)
	if err != nil {
		if notFound(err) {
			return err
		}
		return fmt.Errorf("dnf install %s: %w (output: %s)", pkg, err, strings.TrimSpace(out))
	}
	return nil
}

func (d *Dnf) InstallCommand(pkg string) (string, []string, error) {
	name, args := elevate("dnf", []string{"install", "-y", d.resolve(pkg)})
	return name, args, nil
}

func (d *Dnf) NeedsPrivilege() bool { return true }
