package hardening

import (
	"context"
	"fmt"
	"strings"

	"project-revina/internal/engine"
)

// buildGatekeeper enforces macOS Gatekeeper (spctl assessments) on.
func buildGatekeeper(r engine.Runner) (engine.Step, error) {
	check := func(ctx context.Context) (engine.State, error) {
		out, err := r.Run(ctx, "spctl", "--status")
		if err != nil {
			if notFoundRunner(err) {
				return 0, fmt.Errorf("spctl: %w", err)
			}
			return engine.Missing, nil
		}
		if strings.Contains(out, "assessments enabled") {
			return engine.Satisfied, nil
		}
		return engine.Missing, nil
	}

	apply := func(ctx context.Context) error {
		name, args := elevate("spctl", []string{"--master-enable"})
		if _, err := r.Run(ctx, name, args...); err != nil {
			return fmt.Errorf("spctl --master-enable: %w", err)
		}
		return nil
	}

	return NewRuleStep("gatekeeper", "Gatekeeper assessments enabled",
		"CIS macOS 2.6", "spctl --master-enable", true, check, apply), nil
}

// buildRemoteLogin disables macOS Remote Login (SSH). It refuses over an SSH
// session to avoid locking the remote user out.
func buildRemoteLogin(r engine.Runner) (engine.Step, error) {
	check := func(ctx context.Context) (engine.State, error) {
		out, err := r.Run(ctx, "systemsetup", "-getremotelogin")
		if err != nil {
			if notFoundRunner(err) {
				return 0, fmt.Errorf("systemsetup: %w", err)
			}
			return engine.Missing, nil
		}
		if strings.Contains(out, "Remote Login: Off") {
			return engine.Satisfied, nil
		}
		return engine.Missing, nil
	}

	apply := func(ctx context.Context) error {
		if isSSHSession() {
			return fmt.Errorf("refusing to disable Remote Login over an SSH session (would risk locking you out)")
		}
		name, args := elevate("systemsetup", []string{"-setremotelogin", "off"})
		if _, err := r.Run(ctx, name, args...); err != nil {
			return fmt.Errorf("systemsetup -setremotelogin off: %w", err)
		}
		return nil
	}

	return NewRuleStep("remote-login", "Disable SSH Remote Login",
		"CIS macOS 2.5.1", "systemsetup -setremotelogin off", true, check, apply), nil
}
