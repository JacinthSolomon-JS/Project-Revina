package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"project-revina/internal/engine"
	"project-revina/internal/engine/translators"
)

const testProfile = "name: test\ntasks:\n  - id: pkg\n    kind: package\n    packages:\n      - ripgrep\n"

func writeProfile(t *testing.T) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	path = filepath.Join(dir, "profile.yaml")
	if err := os.WriteFile(path, []byte(testProfile), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return dir, path
}

// overrideFactory replaces the OS translator with a fake-runner-backed brew so
// tests never touch a real package manager. It mutates package-level globals,
// so tests using it must not run in parallel.
func overrideFactory(t *testing.T, resp map[string]engine.FakeResponse) {
	t.Helper()
	prev := translatorFactory
	translatorFactory = func(_ context.Context, _ engine.Runner) (engine.Translator, error) {
		return translators.NewBrew(engine.NewFakeRunner(resp))
	}
	t.Cleanup(func() { translatorFactory = prev })
}

// forceTerminal makes the CLI believe stdin is (or is not) a TTY.
func forceTerminal(t *testing.T, v bool) {
	t.Helper()
	prev := stdinTerminal
	stdinTerminal = func() bool { return v }
	t.Cleanup(func() { stdinTerminal = prev })
}

func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := rootCmd()
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestRunDryRunPrintsPlanWithoutChanges(t *testing.T) {
	overrideFactory(t, nil)
	dir, path := writeProfile(t)

	out, err := execute(t, "--profile", path, "--dry-run")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(out, "brew install ripgrep") || !strings.Contains(out, "no changes made") {
		t.Errorf("dry-run output missing plan:\n%s", out)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "state.json")); statErr == nil {
		t.Error("dry-run must not write a state file")
	}
}

func TestRunRefusesWithoutYesWhenNotATerminal(t *testing.T) {
	overrideFactory(t, nil)
	forceTerminal(t, false)
	_, path := writeProfile(t)

	_, err := execute(t, "--profile", path)
	if err == nil {
		t.Fatal("expected refusal when not a terminal and no --yes")
	}
	if !strings.Contains(err.Error(), "without --non-interactive or --yes") {
		t.Errorf("error = %v, want a clear refusal message", err)
	}
}

func TestRunAppliesWithYes(t *testing.T) {
	overrideFactory(t, map[string]engine.FakeResponse{
		"brew list --versions ripgrep": {Err: errors.New("not installed")},
		"brew install ripgrep":         {},
	})
	forceTerminal(t, false)
	dir, path := writeProfile(t)
	stateFile := filepath.Join(dir, "state.json")

	out, err := execute(t, "--profile", path, "--yes", "--state", stateFile)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(out, "APPLIED  pkg:ripgrep") {
		t.Errorf("output missing APPLIED line:\n%s", out)
	}
	if !strings.Contains(out, "0 satisfied, 1 applied") {
		t.Errorf("output missing summary:\n%s", out)
	}
	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("state file not written: %v", err)
	}
	if !strings.Contains(string(data), `"step_id":"pkg:ripgrep"`) || !strings.Contains(string(data), `"applied":true`) {
		t.Errorf("state file missing expected audit entry:\n%s", data)
	}
}

func TestRunSkipWhenAlreadySatisfied(t *testing.T) {
	overrideFactory(t, map[string]engine.FakeResponse{
		"brew list --versions ripgrep": {Output: "ripgrep 14.1.0"},
	})
	forceTerminal(t, false)
	dir, path := writeProfile(t)

	out, err := execute(t, "--profile", path, "--yes", "--state", filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(out, "already satisfied") {
		t.Errorf("output missing satisfied line:\n%s", out)
	}
	if !strings.Contains(out, "1 satisfied, 0 applied") {
		t.Errorf("output missing summary:\n%s", out)
	}
}

func TestAuditReportsOnly(t *testing.T) {
	overrideFactory(t, map[string]engine.FakeResponse{
		"brew list --versions ripgrep": {Output: "ripgrep 14.1.0"},
	})
	_, path := writeProfile(t)

	out, err := execute(t, "audit", "--profile", path)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(out, "satisfied pkg:ripgrep") {
		t.Errorf("output missing per-step state:\n%s", out)
	}
	if !strings.Contains(out, "no changes made") {
		t.Errorf("output missing audit-only marker:\n%s", out)
	}
}

func TestAuditReportsMissing(t *testing.T) {
	overrideFactory(t, map[string]engine.FakeResponse{
		"brew list --versions ripgrep": {Err: errors.New("not installed")},
	})
	_, path := writeProfile(t)

	out, err := execute(t, "audit", "--profile", path)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(out, "missing pkg:ripgrep") {
		t.Errorf("output missing missing state:\n%s", out)
	}
}

func TestLoadProfilePrefersExistingFileOverEmbedded(t *testing.T) {
	dir := t.TempDir()
	local := "name: local-setup\ntasks:\n  - id: cli\n    kind: package\n    packages:\n      - ripgrep\n"
	if err := os.WriteFile(filepath.Join(dir, "dev.yaml"), []byte(local), 0o644); err != nil {
		t.Fatalf("write local dev.yaml: %v", err)
	}
	t.Chdir(dir)

	p, err := loadProfile("dev.yaml")
	if err != nil {
		t.Fatalf("loadProfile: %v", err)
	}
	if p.Name != "local-setup" {
		t.Errorf("expected local file to win over embedded name, got %q", p.Name)
	}
}

func TestLoadProfileFallsBackToEmbedded(t *testing.T) {
	p, err := loadProfile("dev.yaml")
	if err != nil {
		t.Fatalf("loadProfile: %v", err)
	}
	if p.Name != "dev" {
		t.Errorf("expected embedded dev profile, got %q", p.Name)
	}
}
