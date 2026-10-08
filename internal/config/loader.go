package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// packageNameRe bounds what a profile may name as a package. Logical names are
// passed as argv to apt/dnf/pacman/winget/scoop/choco/brew, sometimes via
// sudo, so anything option-like (leading "-", whitespace, path separators,
// shell metacharacters) is rejected up front.
var packageNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+_.:=@-]*$`)

// Load reads, parses, and validates a profile from disk.
func Load(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read profile %s: %w", path, err)
	}
	return Parse(data)
}

// Parse decodes a profile from YAML using strict field matching so that typos
// in profile keys are rejected instead of silently ignored.
func Parse(data []byte) (*Profile, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	var p Profile
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("parse profile: %w", err)
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate checks structural invariants of a profile. It returns an error
// aggregating every problem found so users can fix a profile in one pass.
func (p *Profile) Validate() error {
	var errs []error

	if p.Name == "" {
		errs = append(errs, errors.New("profile name is required"))
	}
	if len(p.Tasks) == 0 {
		errs = append(errs, errors.New("profile must define at least one task"))
	}

	seen := make(map[string]bool, len(p.Tasks))
	for i, t := range p.Tasks {
		switch {
		case t.ID == "":
			errs = append(errs, fmt.Errorf("tasks[%d]: id is required", i))
		case seen[t.ID]:
			errs = append(errs, fmt.Errorf("tasks: duplicate id %q", t.ID))
		}
		seen[t.ID] = true

		switch t.Kind {
		case KindPackage:
			if len(t.Packages) == 0 {
				errs = append(errs, fmt.Errorf("tasks[%d] (%q): package task requires at least one package", i, t.ID))
			}
			for j, pkg := range t.Packages {
				if strings.TrimSpace(pkg) == "" {
					errs = append(errs, fmt.Errorf("tasks[%d] (%q): packages[%d] is empty", i, t.ID, j))
					continue
				}
				if !packageNameRe.MatchString(pkg) {
					errs = append(errs, fmt.Errorf("tasks[%d] (%q): packages[%d] %q is not a valid package name (must match %s)", i, t.ID, j, pkg, packageNameRe))
				}
			}
		case KindHardening:
			if len(t.Rules) == 0 {
				errs = append(errs, fmt.Errorf("tasks[%d] (%q): hardening task requires at least one rule", i, t.ID))
			}
			for j, rule := range t.Rules {
				if strings.TrimSpace(rule) == "" {
					errs = append(errs, fmt.Errorf("tasks[%d] (%q): rules[%d] is empty", i, t.ID, j))
				}
			}
		default:
			errs = append(errs, fmt.Errorf("tasks[%d] (%q): unknown kind %q", i, t.ID, t.Kind))
		}
	}

	return errors.Join(errs...)
}
