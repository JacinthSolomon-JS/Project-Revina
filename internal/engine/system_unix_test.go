//go:build unix

package engine

import (
	"os"
	"testing"
)

func TestIsElevated(t *testing.T) {
	t.Parallel()

	if got, want := IsElevated(), os.Geteuid() == 0; got != want {
		t.Errorf("IsElevated() = %v, want %v", got, want)
	}
}
