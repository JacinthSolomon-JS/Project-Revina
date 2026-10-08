package hardening

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"project-revina/internal/engine"
)

func TestFactoryUnknownRule(t *testing.T) {
	_, err := Factory(engine.NewFakeRunner(nil))("not-a-rule")
	if err == nil {
		t.Fatal("Factory(unknown) returned nil error")
	}
	if !strings.Contains(err.Error(), "unknown hardening rule") {
		t.Errorf("error = %v, want clear message", err)
	}
}

func TestFactoryRuleNotAvailableOnThisOS(t *testing.T) {
	// disable-smbv1 is registered only on windows; on macOS/Linux the factory
	// must skip it (nil step) rather than error, so shared cross-OS profiles work.
	step, err := Factory(engine.NewFakeRunner(nil))("disable-smbv1")
	if err != nil {
		t.Fatalf("Factory returned error for cross-OS rule: %v", err)
	}
	if runtime.GOOS == "windows" {
		if step == nil {
			t.Error("disable-smbv1 should be registered on windows")
		}
	} else if step != nil {
		t.Error("disable-smbv1 must be skipped (nil step) outside windows")
	}
}

func TestGatekeeperCheckAndApply(t *testing.T) {
	fr := engine.NewFakeRunner(map[string]engine.FakeResponse{
		"spctl --status": {Output: "assessments enabled"},
	})
	step, err := Factory(fr)("gatekeeper")
	if err != nil {
		t.Fatalf("Factory(gatekeeper): %v", err)
	}
	if step == nil {
		t.Skip("gatekeeper not registered on this OS")
	}
	state, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if state != engine.Satisfied {
		t.Errorf("state = %v, want Satisfied", state)
	}

	// Apply records the spctl --master-enable invocation.
	fr2 := engine.NewFakeRunner(nil)
	step2, _ := Factory(fr2)("gatekeeper")
	if err := step2.Apply(context.Background()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !callsContain(fr2.Calls(), "--master-enable") {
		t.Errorf("Apply did not run spctl --master-enable: %+v", fr2.Calls())
	}
}

func TestRemoteLoginRefusesOverSSH(t *testing.T) {
	step, err := Factory(engine.NewFakeRunner(nil))("remote-login")
	if err != nil {
		t.Fatalf("Factory(remote-login): %v", err)
	}
	if step == nil {
		t.Skip("remote-login not registered on this OS")
	}
	t.Setenv("SSH_CONNECTION", "10.0.0.1 22 10.0.0.2 54321")
	err = step.Apply(context.Background())
	if err == nil {
		t.Fatal("Apply over SSH session returned nil error, want refusal")
	}
	if !strings.Contains(err.Error(), "SSH session") {
		t.Errorf("error = %v, want SSH refusal message", err)
	}
}

func TestSSHDDropInCheckAndRollback(t *testing.T) {
	dir := t.TempDir()
	prev := sshdDropinPath
	sshdDropinPath = filepath.Join(dir, "99-devsec.conf")
	t.Cleanup(func() { sshdDropinPath = prev })

	fr := engine.NewFakeRunner(map[string]engine.FakeResponse{
		"sshd -t": {Err: errors.New("bad config")},
	})
	step, err := Factory(fr)("sshd-password-auth")
	if err != nil {
		t.Fatalf("Factory(sshd-password-auth): %v", err)
	}
	if step == nil {
		t.Skip("sshd-password-auth not registered on this OS")
	}
	state, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if state != engine.Missing {
		t.Errorf("state = %v, want Missing (no drop-in yet)", state)
	}

	// Validation failure must roll the file back and error.
	err = step.Apply(context.Background())
	if err == nil {
		t.Fatal("Apply returned nil error when sshd -t fails")
	}
	if _, statErr := os.Stat(sshdDropinPath); statErr == nil {
		t.Error("drop-in still present after failed sshd -t (rollback failed)")
	}

	// With valid config the drop-in appears and Check becomes Satisfied.
	fr2 := engine.NewFakeRunner(map[string]engine.FakeResponse{
		"sshd -t": {},
	})
	step2, _ := Factory(fr2)("sshd-password-auth")
	if err := step2.Apply(context.Background()); err != nil {
		t.Fatalf("Apply (valid): %v", err)
	}
	if _, statErr := os.Stat(sshdDropinPath); statErr != nil {
		t.Fatalf("drop-in not created: %v", statErr)
	}
	state2, err := step2.Check(context.Background())
	if err != nil {
		t.Fatalf("Check after apply: %v", err)
	}
	if state2 != engine.Satisfied {
		t.Errorf("state after apply = %v, want Satisfied", state2)
	}
}

func TestSysctlCheckDrift(t *testing.T) {
	dir := t.TempDir()
	prev := sysctlConfPath
	sysctlConfPath = filepath.Join(dir, "60-devsec.conf")
	t.Cleanup(func() { sysctlConfPath = prev })

	if err := os.WriteFile(sysctlConfPath, []byte(sysctlContent), 0o644); err != nil {
		t.Fatalf("write drop-in: %v", err)
	}

	// One parameter drifted.
	fr := engine.NewFakeRunner(map[string]engine.FakeResponse{
		"sysctl -n kernel.randomize_va_space":   {Output: "2"},
		"sysctl -n net.ipv4.tcp_syncookies":     {Output: "0"},
		"sysctl -n net.ipv4.conf.all.rp_filter": {Output: "1"},
	})
	step, err := Factory(fr)("sysctl-secure")
	if err != nil {
		t.Fatalf("Factory(sysctl-secure): %v", err)
	}
	if step == nil {
		t.Skip("sysctl-secure not registered on this OS")
	}
	state, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if state != engine.Drifted {
		t.Errorf("state = %v, want Drifted", state)
	}
}

func TestDropInRefusesForeignFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "conf")
	if err := os.WriteFile(path, []byte("user: 1"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	d := dropIn{path: path, content: "devsec: 1", header: "# devsec"}
	if _, _, err := d.write(); err == nil {
		t.Fatal("write of a foreign file returned nil error")
	}
}

func TestDropInRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("precious"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(dir, "conf")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	d := dropIn{path: link, content: "devsec: 1", header: "# devsec"}
	if _, _, err := d.write(); err == nil {
		t.Fatal("write through a symlink returned nil error")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(data) != "precious" {
		t.Errorf("target modified through symlink: %q", data)
	}
}

func TestRegValueParsing(t *testing.T) {
	out := "HKEY_LOCAL_MACHINE\\X\n    SMB1    REG_DWORD    0x0\n"
	val, ok := regValue(out, "SMB1")
	if !ok || val != "0x0" {
		t.Errorf("regValue = (%q, %v), want (0x0, true)", val, ok)
	}
}

func TestSSHSessionDetection(t *testing.T) {
	t.Setenv("SSH_CONNECTION", "1.2.3.4 22 5.6.7.8 12345")
	if !isSSHSession() {
		t.Error("isSSHSession = false with SSH_CONNECTION set")
	}
}

func callsContain(calls []engine.RunCall, want string) bool {
	for _, c := range calls {
		for _, a := range c.Args {
			if a == want {
				return true
			}
		}
	}
	return false
}
