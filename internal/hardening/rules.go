package hardening

import (
	"context"
	"fmt"
	"strings"

	"project-revina/internal/engine"
)

// Paths are package vars so tests can point the drop-in rules at temp files
// without ever touching the host.
var (
	sysctlConfPath = "/etc/sysctl.d/60-devsec.conf"
	sshdDropinPath = "/etc/ssh/sshd_config.d/99-devsec.conf"
)

const sysctlHeader = "# devsec hardening"

const sysctlContent = sysctlHeader + `
kernel.randomize_va_space = 2
net.ipv4.tcp_syncookies = 1
net.ipv4.conf.all.rp_filter = 1
`

// buildSysctl enforces secure kernel parameters via a sysctl drop-in
// (CIS Debian 1.5.x / kernel hardening family).
func buildSysctl(r engine.Runner) (engine.Step, error) {
	drop := dropIn{path: sysctlConfPath, content: sysctlContent, header: sysctlHeader}
	want := map[string]string{
		"kernel.randomize_va_space":   "2",
		"net.ipv4.tcp_syncookies":     "1",
		"net.ipv4.conf.all.rp_filter": "1",
	}

	check := func(ctx context.Context) (engine.State, error) {
		if _, err := readFile(drop.path); err != nil {
			return engine.Missing, nil
		}
		drifted := false
		for key, wantVal := range want {
			out, err := r.Run(ctx, "sysctl", "-n", key)
			if err != nil {
				if notFoundRunner(err) {
					return 0, fmt.Errorf("sysctl: %w", err)
				}
				drifted = true
				continue
			}
			if strings.TrimSpace(out) != wantVal {
				drifted = true
			}
		}
		if drifted {
			return engine.Drifted, nil
		}
		return engine.Satisfied, nil
	}

	apply := func(ctx context.Context) error {
		name, args := elevate("sysctl", []string{"-p", drop.path})
		if _, changed, err := drop.write(); err != nil {
			return err
		} else if !changed {
			return nil
		}
		if _, err := r.Run(ctx, name, args...); err != nil {
			return fmt.Errorf("apply sysctl values: %w", err)
		}
		return nil
	}

	return NewRuleStep("sysctl-secure", "Secure kernel parameters",
		"CIS 1.5", "write "+sysctlConfPath+" + sysctl -p", true, check, apply), nil
}

const sshdHeader = "# devsec hardening"

const sshdContent = sshdHeader + `
PasswordAuthentication no
`

// buildSSHD enforces password-less SSH login via a drop-in file
// (CIS 5.3.x / sshd_config hardening family). Applies only when it can
// validate with `sshd -t` first.
func buildSSHD(r engine.Runner) (engine.Step, error) {
	drop := dropIn{path: sshdDropinPath, content: sshdContent, header: sshdHeader}

	check := func(ctx context.Context) (engine.State, error) {
		data, err := readFile(drop.path)
		if err != nil {
			return engine.Missing, nil
		}
		if len(data) == 0 {
			return engine.Missing, nil
		}
		if strings.Contains(string(data), "PasswordAuthentication no") {
			return engine.Satisfied, nil
		}
		return engine.Drifted, nil
	}

	apply := func(ctx context.Context) error {
		if isSSHSession() {
			return fmt.Errorf("refusing to disable SSH password login over an SSH session (would risk locking you out)")
		}
		orig, changed, err := drop.write()
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		if out, err := r.Run(ctx, "sshd", "-t"); err != nil {
			_ = drop.restore(orig)
			return fmt.Errorf("sshd -t failed; change rolled back: %w (%s)", err, strings.TrimSpace(out))
		}
		return nil
	}

	return NewRuleStep("sshd-password-auth", "Disable SSH password authentication",
		"CIS 5.3.2", "write "+sshdDropinPath+" || sshd -t", true, check, apply), nil
}

// buildUFW enables the UFW firewall (CIS 3.x firewall family).
func buildUFW(r engine.Runner) (engine.Step, error) {
	check := func(ctx context.Context) (engine.State, error) {
		out, err := r.Run(ctx, "ufw", "status")
		if err != nil {
			if notFoundRunner(err) {
				return 0, fmt.Errorf("ufw: %w", err)
			}
			return engine.Missing, nil
		}
		if strings.Contains(out, "Status: active") {
			return engine.Satisfied, nil
		}
		return engine.Missing, nil
	}

	apply := func(ctx context.Context) error {
		if isSSHSession() {
			return fmt.Errorf("refusing to enable the firewall over an SSH session unless an allow rule for your SSH port is already in place")
		}
		// Allow OpenSSH before enabling so a firewall enable from the local
		// console never severs remote SSH access for other users.
		name, args := elevate("ufw", []string{"allow", "OpenSSH"})
		if out, err := r.Run(ctx, name, args...); err != nil {
			return fmt.Errorf("ufw allow OpenSSH: %w (%s)", err, strings.TrimSpace(out))
		}
		name, args = elevate("ufw", []string{"--force", "enable"})
		out, err := r.Run(ctx, name, args...)
		if err != nil {
			return fmt.Errorf("ufw --force enable: %w (%s)", err, strings.TrimSpace(out))
		}
		return nil
	}

	return NewRuleStep("ufw-enable", "Enable UFW firewall",
		"CIS 4.x/3.5", "ufw allow OpenSSH && ufw --force enable", true, check, apply), nil
}
