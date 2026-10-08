package config

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()

	valid := []byte(`
name: dev
description: Core CLI utilities
tasks:
  - id: cli
    kind: package
    packages:
      - ripgrep
      - fzf
`)

	p, err := Parse(valid)
	if err != nil {
		t.Fatalf("Parse(valid) returned error: %v", err)
	}
	if p.Name != "dev" {
		t.Errorf("Name = %q, want %q", p.Name, "dev")
	}
	if len(p.Tasks) != 1 || len(p.Tasks[0].Packages) != 2 {
		t.Errorf("unexpected task shape: %+v", p.Tasks)
	}
}

// TestParseAllowsRealPackageNames guards against over-strict validation: the
// names below are used by the winget/scoop/apt mapping tables.
func TestParseAllowsRealPackageNames(t *testing.T) {
	t.Parallel()
	for _, pkg := range []string{"sharkdp.bat", "BurntSushi.ripgrep.MSVC", "libfoo:amd64", "rustup", "pkg=1.2.3"} {
		y := []byte("name: p\ntasks:\n  - id: a\n    kind: package\n    packages: [" + pkg + "]\n")
		if _, err := Parse(y); err != nil {
			t.Errorf("Parse(package %q) rejected a valid name: %v", pkg, err)
		}
	}
}

func TestParseRejectsInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name:    "missing name",
			yaml:    "tasks:\n  - id: a\n    kind: package\n    packages: [x]\n",
			wantErr: "name is required",
		},
		{
			name:    "empty tasks",
			yaml:    "name: p\n",
			wantErr: "at least one task",
		},
		{
			name:    "duplicate task id",
			yaml:    "name: p\ntasks:\n  - id: a\n    kind: package\n    packages: [x]\n  - id: a\n    kind: package\n    packages: [y]\n",
			wantErr: `duplicate id "a"`,
		},
		{
			name:    "unknown kind",
			yaml:    "name: p\ntasks:\n  - id: a\n    kind: explode\n    packages: [x]\n",
			wantErr: `unknown kind "explode"`,
		},
		{
			name:    "package task without packages",
			yaml:    "name: p\ntasks:\n  - id: a\n    kind: package\n",
			wantErr: "requires at least one package",
		},
		{
			name:    "empty package entry",
			yaml:    "name: p\ntasks:\n  - id: a\n    kind: package\n    packages: ['', x]\n",
			wantErr: "is empty",
		},
		{
			name:    "unknown field rejected",
			yaml:    "name: p\ntasks: []\npkgs: [nope]\n",
			wantErr: "field pkgs not found",
		},
		{
			name:    "package name leading dash",
			yaml:    "name: p\ntasks:\n  - id: a\n    kind: package\n    packages: ['-y']\n",
			wantErr: "not a valid package name",
		},
		{
			name:    "package name with option flag",
			yaml:    "name: p\ntasks:\n  - id: a\n    kind: package\n    packages: ['--allow-downgrades=1']\n",
			wantErr: "not a valid package name",
		},
		{
			name:    "package name with space",
			yaml:    "name: p\ntasks:\n  - id: a\n    kind: package\n    packages: ['evil pkg']\n",
			wantErr: "not a valid package name",
		},
		{
			name:    "package name with shell metachar",
			yaml:    "name: p\ntasks:\n  - id: a\n    kind: package\n    packages: ['x;rm -rf /']\n",
			wantErr: "not a valid package name",
		},
		{
			name:    "package name with slash",
			yaml:    "name: p\ntasks:\n  - id: a\n    kind: package\n    packages: ['../evil']\n",
			wantErr: "not a valid package name",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := Parse([]byte(tc.yaml))
			if err == nil {
				t.Fatalf("Parse() returned nil error, want error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err, tc.wantErr)
			}
		})
	}
}
