package translators

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"project-revina/internal/engine"
)

func newTrans(t *testing.T, manager string, fr engine.Runner) engine.Translator {
	t.Helper()
	switch manager {
	case "apt":
		tr, err := NewApt(fr)
		if err != nil {
			t.Fatalf("NewApt: %v", err)
		}
		return tr
	case "dnf":
		tr, err := NewDnf(fr)
		if err != nil {
			t.Fatalf("NewDnf: %v", err)
		}
		return tr
	case "pacman":
		tr, err := NewPacman(fr)
		if err != nil {
			t.Fatalf("NewPacman: %v", err)
		}
		return tr
	case "winget":
		tr, err := NewWinget(fr)
		if err != nil {
			t.Fatalf("NewWinget: %v", err)
		}
		return tr
	case "scoop":
		tr, err := NewScoop(fr)
		if err != nil {
			t.Fatalf("NewScoop: %v", err)
		}
		return tr
	case "choco":
		tr, err := NewChoco(fr)
		if err != nil {
			t.Fatalf("NewChoco: %v", err)
		}
		return tr
	default:
		t.Fatalf("unknown manager %q", manager)
		return nil
	}
}

func TestTranslatorsInstalled(t *testing.T) {
	t.Parallel()

	apt := newTrans(t, "apt", engine.NewFakeRunner(map[string]engine.FakeResponse{
		"dpkg -s ripgrep": {Output: "Package: ripgrep\nStatus: install ok installed"},
		"dpkg -s missing": {Err: errors.New("dpkg-query: no packages found matching missing")},
	}))
	if ok, err := apt.Installed(context.Background(), "ripgrep"); err != nil || !ok {
		t.Errorf("apt Installed(ripgrep) = (%v, %v), want true", ok, err)
	}
	if ok, _ := apt.Installed(context.Background(), "missing"); ok {
		t.Error("apt Installed(missing) should be false when dpkg -s fails")
	}

	pac := newTrans(t, "pacman", engine.NewFakeRunner(map[string]engine.FakeResponse{
		"pacman -Q ripgrep": {Output: "ripgrep 14.1.0"},
	}))
	ver, err := pac.Version(context.Background(), "ripgrep")
	if err != nil {
		t.Fatalf("pacman Version returned error: %v", err)
	}
	if ver != "14.1.0" {
		t.Errorf("pacman Version = %q, want 14.1.0", ver)
	}

	scoop := newTrans(t, "scoop", engine.NewFakeRunner(map[string]engine.FakeResponse{
		"scoop list fzf": {Output: "fzf 0.55.0"},
	}))
	if ok, err := scoop.Installed(context.Background(), "fzf"); err != nil || !ok {
		t.Errorf("scoop Installed = (%v, %v), want true", ok, err)
	}

	choco := newTrans(t, "choco", engine.NewFakeRunner(map[string]engine.FakeResponse{
		"choco list --local-only --exact ripgrep": {Output: "ripgrep 14.1.0\n1 packages installed."},
	}))
	if ok, err := choco.Installed(context.Background(), "ripgrep"); err != nil || !ok {
		t.Errorf("choco Installed = (%v, %v), want true", ok, err)
	}
}

func TestTranslatorsResolveMappings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		manager string
		logical string
		wantCmd string
	}{
		{"apt", "fd", "sudo apt-get install -y fd-find"},
		{"apt", "rg", "sudo apt-get install -y ripgrep"},
		{"dnf", "fd", "sudo dnf install -y fd-find"},
		{"pacman", "rg", "sudo pacman -S --noconfirm ripgrep"},
		{"winget", "bat", "winget install --id sharkdp.bat --silent --accept-package-agreements --accept-source-agreements"},
		{"scoop", "fd", "scoop install fd"},
		{"choco", "rg", "choco install ripgrep -y"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.manager+"/"+tc.logical, func(t *testing.T) {
			t.Parallel()
			fr := engine.NewFakeRunner(nil)
			trans := newTrans(t, tc.manager, fr)
			if err := trans.Install(context.Background(), tc.logical); err != nil {
				t.Fatalf("Install returned error: %v", err)
			}
			found := false
			for _, c := range fr.Calls() {
				if strings.Join(append([]string{c.Name}, c.Args...), " ") == tc.wantCmd {
					found = true
				}
			}
			if !found {
				t.Errorf("runner did not run %q; calls: %+v", tc.wantCmd, fr.Calls())
			}
		})
	}
}

func TestTranslatorsPropagateMissingBinary(t *testing.T) {
	t.Parallel()

	for _, manager := range []string{"apt", "dnf", "pacman", "winget", "scoop", "choco"} {
		manager := manager
		t.Run(manager, func(t *testing.T) {
			t.Parallel()
			// Every command the manager runs returns exec.ErrNotFound.
			trans := newTrans(t, manager, &alwaysNotFound{})
			if _, err := trans.Installed(context.Background(), "anything"); err == nil {
				t.Error("Installed returned nil error, want missing-binary error")
			}
		})
	}
}

// alwaysNotFound is a Runner whose every invocation reports a missing binary.
type alwaysNotFound struct{}

func (alwaysNotFound) Run(context context.Context, name string, args ...string) (string, error) {
	return "", exec.ErrNotFound
}

func TestDetectLinuxPM(t *testing.T) {
	t.Parallel()

	fr := engine.NewFakeRunner(map[string]engine.FakeResponse{
		"sh -c if command -v apt-get >/dev/null 2>&1; then echo apt; elif command -v dnf >/dev/null 2>&1; then echo dnf; elif command -v pacman >/dev/null 2>&1; then echo pacman; fi": {Output: "apt\n"},
	})
	pm, err := engine.DetectLinuxPM(context.Background(), fr)
	if err != nil {
		t.Fatalf("DetectLinuxPM returned error: %v", err)
	}
	if pm != "apt" {
		t.Errorf("DetectLinuxPM = %q, want apt", pm)
	}
}

func TestDetectLinuxPMUnsupported(t *testing.T) {
	t.Parallel()

	fr := engine.NewFakeRunner(nil)
	_, err := engine.DetectLinuxPM(context.Background(), fr)
	if err == nil {
		t.Fatal("DetectLinuxPM returned nil error for no manager")
	}
	if !strings.Contains(err.Error(), "no supported package manager") {
		t.Errorf("error = %v, want a helpful message", err)
	}
}
