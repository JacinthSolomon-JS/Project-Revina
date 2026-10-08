package engine

import "context"

// Translator adapts a logical package name to a concrete OS package manager.
// Each manager backend (brew, apt, winget, ...) implements this interface.
// Logical-name -> manager-name mapping lives in YAML tables, not in Go code.
type Translator interface {
	// Name is the human-readable manager name (e.g. "brew").
	Name() string
	// Installed reports whether pkg is currently installed.
	Installed(ctx context.Context, pkg string) (bool, error)
	// Install installs pkg. It must be safe to re-run.
	Install(ctx context.Context, pkg string) error
	// Version returns the installed version of pkg, or ("", nil) if unknown.
	Version(ctx context.Context, pkg string) (string, error)
	// InstallCommand returns the exact command Apply would run, for Describe and
	// dry-run output.
	InstallCommand(pkg string) (name string, args []string, err error)
	// NeedsPrivilege reports whether Install requires elevation.
	NeedsPrivilege() bool
}
