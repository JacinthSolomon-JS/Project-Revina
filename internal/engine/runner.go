package engine

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// Runner executes commands. ALL command execution in the project must go
// through a Runner — never call os/exec directly outside this package. That is
// what makes dry-run possible and translators unit-testable against recorded
// commands.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (stdoutAndStderr string, err error)
}

// ExecRunner is the real Runner backed by os/exec.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	// Sanitized output prevents terminal-control escaping from a package
	// manager's stdout from reaching logs, the audit store, or the TUI.
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	out = sanitizeOut(out)
	if err != nil {
		return string(out), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return string(out), nil
}

// sanitizeOut strips control characters and ANSI escape sequences from command
// output while keeping the newlines and tabs parsers rely on. This is defense
// against log/audit/terminal injection via hostile command output.
func sanitizeOut(out []byte) []byte {
	clean := make([]byte, 0, len(out))
	i := 0
	for i < len(out) {
		b := out[i]
		switch {
		case b == 0x1b: // ESC: skip the whole escape sequence
			if i+1 < len(out) && out[i+1] == '[' {
				i += 2
				for i < len(out) {
					c := out[i]
					i++
					if c >= 0x40 && c <= 0x7e {
						break
					}
				}
				continue
			}
			i++
			continue
		case b == '\r' || (b < 0x20 && b != '\n' && b != '\t'):
			i++
			continue
		default:
			clean = append(clean, b)
			i++
		}
	}
	return clean
}

// ErrNotFound is exported so callers can distinguish "binary missing entirely"
// from "command ran and reported a failure".
var ErrNotFound = exec.ErrNotFound

// FakeRunner records every invocation and returns scripted responses. It is the
// test double used across translator and step tests; the zero value returns
// success with empty output.
type FakeRunner struct {
	mu    sync.Mutex
	calls []RunCall
	resp  map[string]FakeResponse
}

// RunCall is one recorded Runner invocation.
type RunCall struct {
	Name string
	Args []string
}

// FakeResponse is the scripted outcome for a given command line.
type FakeResponse struct {
	Output string
	Err    error
}

// NewFakeRunner returns a FakeRunner prepared with scripted responses keyed by
// full command line, e.g. "brew list --versions ripgrep".
func NewFakeRunner(responses map[string]FakeResponse) *FakeRunner {
	return &FakeRunner{resp: responses}
}

func (f *FakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	key := strings.Join(append([]string{name}, args...), " ")

	f.mu.Lock()
	f.calls = append(f.calls, RunCall{Name: name, Args: args})
	resp, ok := f.resp[key]
	f.mu.Unlock()

	if !ok {
		return "", nil
	}
	return resp.Output, resp.Err
}

// Calls returns a copy of the recorded invocations.
func (f *FakeRunner) Calls() []RunCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]RunCall(nil), f.calls...)
}
