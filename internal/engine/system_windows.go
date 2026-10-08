//go:build windows

package engine

// IsElevated reports whether the current process has administrative rights. A
// real token check (and runas self-elevation) lands with Windows support; for
// now the plan never assumes elevation is available on Windows.
func IsElevated() bool { return false }
