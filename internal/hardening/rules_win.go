package hardening

import (
	"context"
	"fmt"
	"strings"

	"project-revina/internal/engine"
)

const (
	smbv1Key    = `HKLM\SYSTEM\CurrentControlSet\Services\LanmanServer\Parameters`
	psPolicyKey = `HKCU\Software\Microsoft\PowerShell\1\ShellIds\Microsoft.PowerShell`
)

// regValue extracts the data column from `reg query` output for a named value.
func regValue(out, name string) (string, bool) {
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, name) {
			continue
		}
		fields := strings.Fields(line)
		for i, f := range fields {
			if strings.EqualFold(strings.TrimSpace(f), name) && i+2 < len(fields) {
				return fields[i+2], true
			}
		}
	}
	return "", false
}

// buildDisableSMBv1 disables the SMBv1 protocol (CIS Windows 2.3.9.1).
func buildDisableSMBv1(r engine.Runner) (engine.Step, error) {
	check := func(ctx context.Context) (engine.State, error) {
		out, err := r.Run(ctx, "reg", "query", smbv1Key, "/v", "SMB1")
		if err != nil {
			if notFoundRunner(err) {
				return 0, fmt.Errorf("reg: %w", err)
			}
			return engine.Missing, nil
		}
		val, ok := regValue(out, "SMB1")
		if ok && strings.TrimSpace(val) == "0x0" {
			return engine.Satisfied, nil
		}
		return engine.Missing, nil
	}

	apply := func(ctx context.Context) error {
		// Admin-only, but there is no sudo on Windows: NeedsPrivilege flags it
		// and the executor warns when the process is not elevated.
		if _, err := r.Run(ctx, "reg", "add", smbv1Key, "/v", "SMB1", "/t", "REG_DWORD", "/d", "0", "/f"); err != nil {
			return fmt.Errorf("reg add SMB1=0: %w", err)
		}
		return nil
	}

	return NewRuleStep("disable-smbv1", "Disable SMBv1 protocol",
		"CIS 2.3.9.1", "reg add "+smbv1Key+" /v SMB1 /t REG_DWORD /d 0 /f", true, check, apply), nil
}

// buildPowershellPolicy pins the current-user execution policy to Restricted.
func buildPowershellPolicy(r engine.Runner) (engine.Step, error) {
	check := func(ctx context.Context) (engine.State, error) {
		out, err := r.Run(ctx, "reg", "query", psPolicyKey, "/v", "ExecutionPolicy")
		if err != nil {
			if notFoundRunner(err) {
				return 0, fmt.Errorf("reg: %w", err)
			}
			return engine.Missing, nil
		}
		val, ok := regValue(out, "ExecutionPolicy")
		if ok && strings.EqualFold(strings.TrimSpace(val), "Restricted") {
			return engine.Satisfied, nil
		}
		return engine.Missing, nil
	}

	apply := func(ctx context.Context) error {
		if _, err := r.Run(ctx, "reg", "add", psPolicyKey, "/v", "ExecutionPolicy", "/t", "REG_SZ", "/d", "Restricted", "/f"); err != nil {
			return fmt.Errorf("reg add ExecutionPolicy=Restricted: %w", err)
		}
		return nil
	}

	return NewRuleStep("powershell-policy", "PowerShell execution policy Restricted",
		"CIS 2.3.10.2", "reg add "+psPolicyKey+" /v ExecutionPolicy /t REG_SZ /d Restricted /f", false, check, apply), nil
}
