package translators

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"project-revina/internal/engine"
	"project-revina/internal/engine/mapping"
)

// notFound reports whether err is os/exec.ErrNotFound (the binary is missing).
func notFound(err error) bool { return errors.Is(err, exec.ErrNotFound) }

// elevate wraps an install command line with sudo when the process is not
// already running as root. Linux distributors installing globally need root;
// macOS brew runs per-user and never takes this path.
func elevate(name string, args []string) (string, []string) {
	if engine.IsElevated() {
		return name, args
	}
	return "sudo", append([]string{name}, args...)
}

// mapped holds the YAML name-mapping shared by managers.
type mapped struct{ table *mapping.Table }

func loadTable(manager string) (*mapping.Table, error) {
	return mapping.Load(manager)
}

// resolve maps a logical package name to its native name for this manager.
func (m *mapped) resolve(pkg string) string { return m.table.Resolve(pkg) }

// errIfNonNil wraps a command failure. A missing binary keeps its identity so
// callers can distinguish "manager absent" from "command ran and failed".
func errIfNonNil(full string, err error) error {
	if err == nil {
		return nil
	}
	if notFound(err) {
		return err
	}
	return fmt.Errorf("%s: %w", full, err)
}

// splitInstalled separates the first "name version..." pair from a listing
// line. Returns ("", "", false) when the line is not a package listing.
func splitListing(line string) (name, version string, ok bool) {
	fields := strings.Fields(line)
	if len(fields) < 1 {
		return "", "", false
	}
	if len(fields) == 1 {
		return fields[0], "", true
	}
	return fields[0], strings.Join(fields[1:], " "), true
}
