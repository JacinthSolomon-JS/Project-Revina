// Package hardening implements the system-hardening rules. Every rule is an
// engine.Step and follows the project's hardening rules from docs/AGENTS.md:
//
//   - Audit is the default; Apply requires an explicit opt-in (the executor
//     only calls Apply, and the CLI requires --yes).
//   - Changes use drop-in files, not edits to main config files.
//   - Apply validates and aborts on failure; anything changed is backed up and
//     restored on failure.
//   - Rules that can lock a remote user out refuse to run over SSH.
//   - Checks map to CIS/STIG-style IDs where applicable (kept in Rule.Reference).
package hardening

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"project-revina/internal/engine"
)

// RuleBuilder constructs a Step for a logical rule ID.
type RuleBuilder func(r engine.Runner) (engine.Step, error)

// knownRule reports whether id is a canonical rule (implemented somewhere).
func knownRule(id string) bool { _, ok := known[id]; return ok }

// register makes a rule available on the current GOOS. Registries live in
// build-tagged files; every registered builder is defined in this package.
func register(id string, b RuleBuilder) {
	registered[id] = b
}

var (
	registered = map[string]RuleBuilder{}
	known      = map[string]bool{
		"sysctl-secure":      true,
		"sshd-password-auth": true,
		"ufw-enable":         true,
		"gatekeeper":         true,
		"remote-login":       true,
		"disable-smbv1":      true,
		"powershell-policy":  true,
	}
)

// Factory returns an engine.RuleFactory wired to the host OS. Rules that are
// canonical but not implemented on this OS yield a nil Step (skip); unknown
// rule IDs yield an error.
func Factory(r engine.Runner) engine.RuleFactory {
	return func(rule string) (engine.Step, error) {
		build, ok := registered[rule]
		if !ok {
			if knownRule(rule) {
				return nil, nil // recognized, not implemented here
			}
			return nil, fmt.Errorf("unknown hardening rule %q", rule)
		}
		return build(r)
	}
}

// RuleStep is the common Step wrapper: rules are small enough that a single
// struct with function fields is clearer than one type per rule.
type RuleStep struct {
	id      string
	title   string
	ref     string
	snippet string // exact commands/files Apply performs, for Describe
	priv    bool
	checkFn func(ctx context.Context) (engine.State, error)
	applyFn func(ctx context.Context) error
}

// NewRuleStep assembles a RuleStep. snippet must show exactly what Apply does.
func NewRuleStep(id, title, ref, snippet string, priv bool, checkFn func(ctx context.Context) (engine.State, error), applyFn func(ctx context.Context) error) *RuleStep {
	return &RuleStep{id: id, title: title, ref: ref, snippet: snippet, priv: priv, checkFn: checkFn, applyFn: applyFn}
}

func (s *RuleStep) ID() string        { return s.id }
func (s *RuleStep) Title() string     { return s.title }
func (s *RuleStep) Reference() string { return s.ref }

func (s *RuleStep) Check(ctx context.Context) (engine.State, error) { return s.checkFn(ctx) }
func (s *RuleStep) Apply(ctx context.Context) error                 { return s.applyFn(ctx) }
func (s *RuleStep) NeedsPrivilege() bool                            { return s.priv }

func (s *RuleStep) Describe() string {
	if s.ref != "" {
		return fmt.Sprintf("%s [%s] -- %s", s.snippet, s.ref, s.title)
	}
	return fmt.Sprintf("%s -- %s", s.snippet, s.title)
}

// isSSHSession reports whether the current session looks like a remote SSH
// login. Rules that can cut off remote access (firewall, sshd, remote login)
// refuse to apply in this situation.
func isSSHSession() bool {
	for _, k := range []string{"SSH_CONNECTION", "SSH_CLIENT", "SSH_TTY"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return true
		}
	}
	return false
}

// elevate wraps a command line with sudo when not already root. Windows rules
// never use this (there is no sudo); they report NeedsPrivilege instead.
func elevate(name string, args []string) (string, []string) {
	if engine.IsElevated() {
		return name, args
	}
	return "sudo", append([]string{name}, args...)
}

// readFile returns a file's contents, treating a missing file as empty.
func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}

// notFoundRunner reports whether err is os/exec.ErrNotFound (the command is
// absent from the host, distinct from "ran and failed").
func notFoundRunner(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
}
