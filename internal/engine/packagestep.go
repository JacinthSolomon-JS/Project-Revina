package engine

import (
	"context"
	"fmt"
)

// PackageStep is a Step that installs one package through a Translator.
type PackageStep struct {
	id   string
	pkg  string
	tr   Translator
	desc string
}

// NewPackageStep builds a Step from a logical package name. desc is the task
// description for context in dry-run output.
func NewPackageStep(id, pkg string, tr Translator, desc string) *PackageStep {
	return &PackageStep{id: id, pkg: pkg, tr: tr, desc: desc}
}

func (s *PackageStep) ID() string { return s.id }

func (s *PackageStep) Check(ctx context.Context) (State, error) {
	installed, err := s.tr.Installed(ctx, s.pkg)
	if err != nil {
		return 0, fmt.Errorf("check %s: %w", s.pkg, err)
	}
	if installed {
		return Satisfied, nil
	}
	return Missing, nil
}

func (s *PackageStep) Apply(ctx context.Context) error {
	if err := s.tr.Install(ctx, s.pkg); err != nil {
		return fmt.Errorf("install %s: %w", s.pkg, err)
	}
	return nil
}

func (s *PackageStep) Describe() string {
	name, args, err := s.tr.InstallCommand(s.pkg)
	if err != nil {
		return fmt.Sprintf("%s (install %s)", s.pkg, s.tr.Name())
	}
	cmd := name
	for _, a := range args {
		cmd += " " + a
	}
	if s.desc != "" {
		return fmt.Sprintf("%s -- %s", cmd, s.desc)
	}
	return cmd
}

func (s *PackageStep) NeedsPrivilege() bool { return s.tr.NeedsPrivilege() }
