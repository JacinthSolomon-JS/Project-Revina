//go:build unix

package engine

import "os"

// IsElevated reports whether the current process has root privileges. It is
// used only to warn when the plan contains privileged steps, never to escalate.
func IsElevated() bool { return os.Geteuid() == 0 }
