package components

import (
	"context"
	"strings"
	"testing"

	"project-revina/internal/engine"
)

type fakeStep struct {
	id, desc string
	priv     bool
}

func (f *fakeStep) ID() string                                  { return f.id }
func (f *fakeStep) Describe() string                            { return f.desc }
func (f *fakeStep) NeedsPrivilege() bool                        { return f.priv }
func (f *fakeStep) Check(context.Context) (engine.State, error) { return 0, nil }
func (f *fakeStep) Apply(context.Context) error                 { return nil }

func TestHeaderRender(t *testing.T) {
	h := Header{Profile: "dev", OS: "macOS", Count: 3, Elevated: true}
	for _, want := range []string{"devsec", "dev", "macOS", "3", "elevated"} {
		if !strings.Contains(h.Render(), want) {
			t.Errorf("header missing %q: %q", want, h.Render())
		}
	}
}

func TestPlanListRender(t *testing.T) {
	l := PlanList{Steps: []engine.Step{
		&fakeStep{id: "ripgrep", desc: "brew install ripgrep"},
		&fakeStep{id: "sysctl-secure", desc: "write /etc/sysctl.d", priv: true},
	}}
	out := l.Render()
	for _, want := range []string{"ripgrep", "brew install ripgrep", "sysctl-secure", "sudo"} {
		if !strings.Contains(out, want) {
			t.Errorf("plan missing %q in:\n%s", want, out)
		}
	}
}
