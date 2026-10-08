package translators

import (
	"context"
	"fmt"
	"strings"

	"project-revina/internal/engine"
)

// Apt implements engine.Translator for Debian/Ubuntu.
type Apt struct {
	mapped
	r engine.Runner
}

func NewApt(r engine.Runner) (*Apt, error) {
	table, err := loadTable("apt")
	if err != nil {
		return nil, err
	}
	return &Apt{mapped: mapped{table: table}, r: r}, nil
}

func (a *Apt) Name() string { return "apt" }

// Installed uses dpkg, which needs no privileges and reports exit 0 only when
// the package is installed.
func (a *Apt) Installed(ctx context.Context, pkg string) (bool, error) {
	_, err := a.r.Run(ctx, "dpkg", "-s", a.resolve(pkg))
	if err != nil {
		if notFound(err) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

func (a *Apt) Version(ctx context.Context, pkg string) (string, error) {
	out, err := a.r.Run(ctx, "dpkg-query", "-W", "-f=${Version}", a.resolve(pkg))
	if err != nil {
		if notFound(err) {
			return "", err
		}
		return "", nil
	}
	return strings.TrimSpace(out), nil
}

func (a *Apt) Install(ctx context.Context, pkg string) error {
	name, args := elevate("apt-get", []string{"install", "-y", a.resolve(pkg)})
	out, err := a.r.Run(ctx, name, args...)
	if err != nil {
		if notFound(err) {
			return err
		}
		return fmt.Errorf("apt-get install %s: %w (output: %s)", pkg, err, strings.TrimSpace(out))
	}
	return nil
}

func (a *Apt) InstallCommand(pkg string) (string, []string, error) {
	name, args := elevate("apt-get", []string{"install", "-y", a.resolve(pkg)})
	return name, args, nil
}

func (a *Apt) NeedsPrivilege() bool { return true }
