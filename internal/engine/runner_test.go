package engine

import (
	"bytes"
	"strings"
	"testing"
)

func TestSanitizeOut(t *testing.T) {
	out := sanitizeOut([]byte("ok\x1b[31mred\x00\nkeep\tme\rmx"))
	want := "okred\nkeep\tmemx" // CSI, NUL and CR stripped; \n and \t kept
	if !bytes.Equal(out, []byte(want)) {
		t.Errorf("sanitizeOut = %q, want %q", out, want)
	}
}

func TestSanitizeOutKeepsUTF8(t *testing.T) {
	in := []byte("héllo wörld\n")
	if !bytes.Equal(sanitizeOut(in), in) {
		t.Errorf("UTF-8 bytes were mangled: %q", sanitizeOut(in))
	}
}

func TestExecRunnerStripsControlChars(t *testing.T) {
	// `printf` writes a raw ANSI CSI sequence; the Runner must not surface it.
	out, err := (ExecRunner{}).Run(t.Context(), "printf", "a\\033[31mb")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(out, "\x1b") {
		t.Errorf("ExecRunner leaked control byte: %q", out)
	}
	if out != "ab" {
		t.Errorf("output = %q, want %q", out, "ab")
	}
}
