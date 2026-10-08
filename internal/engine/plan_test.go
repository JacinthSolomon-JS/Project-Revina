package engine

import (
	"context"
	"fmt"
	"testing"

	"project-revina/internal/config"
)

func TestBuildPlan(t *testing.T) {
	t.Parallel()

	p := &config.Profile{
		Name: "test",
		Tasks: []config.Task{
			{ID: "a", Kind: config.KindPackage, Packages: []string{"foo", "bar"}},
			{ID: "b", Kind: config.KindPackage, Packages: []string{"baz"}},
		},
	}

	tr := &stubTranslator{name: "stub"}
	steps, err := BuildPlan(p, tr, nil)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("got %d steps, want 3", len(steps))
	}
	want := []string{"a:foo", "a:bar", "b:baz"}
	for i, s := range steps {
		if s.ID() != want[i] {
			t.Errorf("step[%d].ID = %q, want %q", i, s.ID(), want[i])
		}
		if !s.NeedsPrivilege() {
			t.Errorf("step[%d].NeedsPrivilege = false, want true", i)
		}
	}
}

func TestBuildPlanUnknownKind(t *testing.T) {
	t.Parallel()

	p := &config.Profile{
		Name:  "test",
		Tasks: []config.Task{{ID: "a", Kind: config.TaskKind("blorp")}},
	}
	if _, err := BuildPlan(p, &stubTranslator{}, nil); err == nil {
		t.Fatal("BuildPlan returned nil error for unknown kind")
	}
}

func TestBuildPlanHardeningTask(t *testing.T) {
	t.Parallel()

	p := &config.Profile{
		Name: "test",
		Tasks: []config.Task{
			{ID: "secure", Kind: config.KindHardening, Rules: []string{"sysctl-secure", "noop-on-other-os"}},
			{ID: "apps", Kind: config.KindPackage, Packages: []string{"foo"}},
		},
	}
	factory := func(rule string) (Step, error) {
		switch rule {
		case "sysctl-secure":
			return &stubStep{id: "rules:sysctl-secure"}, nil
		case "noop-on-other-os":
			return nil, nil // recognized but not implemented on this OS -> skip
		default:
			return nil, fmt.Errorf("unknown rule %q", rule)
		}
	}

	steps, err := BuildPlan(p, &stubTranslator{}, factory)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("got %d steps, want 2 (skipped OS-noop rule)", len(steps))
	}
	if steps[0].ID() != "rules:sysctl-secure" {
		t.Errorf("step[0].ID = %q, want rules:sysctl-secure", steps[0].ID())
	}
}

func TestBuildPlanHardeningRequiresFactory(t *testing.T) {
	t.Parallel()

	p := &config.Profile{
		Name:  "test",
		Tasks: []config.Task{{ID: "secure", Kind: config.KindHardening, Rules: []string{"sysctl-secure"}}},
	}
	if _, err := BuildPlan(p, &stubTranslator{}, nil); err == nil {
		t.Fatal("BuildPlan with hardening task and nil factory returned nil error")
	}
}

// stubTranslator is a trivial Translator for planner tests.
type stubTranslator struct{ name string }

func (s *stubTranslator) Name() string                                    { return s.name }
func (s *stubTranslator) Installed(context.Context, string) (bool, error) { return false, nil }
func (s *stubTranslator) Install(context.Context, string) error           { return nil }
func (s *stubTranslator) Version(context.Context, string) (string, error) { return "", nil }
func (s *stubTranslator) InstallCommand(string) (string, []string, error) { return "x", nil, nil }
func (s *stubTranslator) NeedsPrivilege() bool                            { return true }

// stubStep is a minimal Step for planner tests.
type stubStep struct{ id string }

func (s *stubStep) ID() string                           { return s.id }
func (s *stubStep) Check(context.Context) (State, error) { return Satisfied, nil }
func (s *stubStep) Apply(context.Context) error          { return nil }
func (s *stubStep) Describe() string                     { return s.id }
func (s *stubStep) NeedsPrivilege() bool                 { return false }
