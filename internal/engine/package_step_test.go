package engine_test

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"project-revina/internal/engine"
	"project-revina/internal/engine/translators"
)

func TestPackageStepCheckAndApply(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		resp      engine.FakeResponse
		wantState engine.State
		wantApply bool
	}{
		{
			name:      "installed",
			resp:      engine.FakeResponse{Output: "ripgrep 14.1.0"},
			wantState: engine.Satisfied,
			wantApply: false,
		},
		{
			name:      "not installed",
			resp:      engine.FakeResponse{Err: errors.New("no such keg")},
			wantState: engine.Missing,
			wantApply: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fr := engine.NewFakeRunner(map[string]engine.FakeResponse{
				"brew list --versions ripgrep": tc.resp,
				"brew install ripgrep":         {},
			})
			tr, err := translators.NewBrew(fr)
			if err != nil {
				t.Fatalf("NewBrew returned error: %v", err)
			}
			step := engine.NewPackageStep("dev:ripgrep", "ripgrep", tr, "Modern CLI")

			state, err := step.Check(context.Background())
			if err != nil {
				t.Fatalf("Check returned error: %v", err)
			}
			if state != tc.wantState {
				t.Errorf("state = %v, want %v", state, tc.wantState)
			}

			if tc.wantApply {
				if err := step.Apply(context.Background()); err != nil {
					t.Fatalf("Apply returned error: %v", err)
				}
			}
			gotApply := false
			for _, c := range fr.Calls() {
				if strings.Join(append([]string{c.Name}, c.Args...), " ") == "brew install ripgrep" {
					gotApply = true
				}
			}
			if gotApply != tc.wantApply {
				t.Errorf("ran brew install = %v, want %v", gotApply, tc.wantApply)
			}
		})
	}
}

func TestPackageStepApplyIdempotent(t *testing.T) {
	t.Parallel()

	fr := engine.NewFakeRunner(nil)
	tr, err := translators.NewBrew(fr)
	if err != nil {
		t.Fatalf("NewBrew returned error: %v", err)
	}
	step := engine.NewPackageStep("dev:ripgrep", "ripgrep", tr, "")
	for i := 0; i < 2; i++ {
		if err := step.Apply(context.Background()); err != nil {
			t.Fatalf("Apply #%d returned error: %v", i+1, err)
		}
	}
	if n := len(fr.Calls()); n != 2 {
		t.Errorf("Apply called %d times, want 2 (check+apply distinction)", n)
	}
}

func TestPackageStepCheckPropagatesMissingBinary(t *testing.T) {
	t.Parallel()

	fr := engine.NewFakeRunner(map[string]engine.FakeResponse{
		"brew list --versions ripgrep": {Err: exec.ErrNotFound},
	})
	tr, err := translators.NewBrew(fr)
	if err != nil {
		t.Fatalf("NewBrew returned error: %v", err)
	}
	step := engine.NewPackageStep("dev:ripgrep", "ripgrep", tr, "")
	if _, err := step.Check(context.Background()); err == nil {
		t.Fatal("Check returned nil error when brew is missing")
	}
}

func TestPrintPlanShowsExactCommand(t *testing.T) {
	t.Parallel()

	fr := engine.NewFakeRunner(nil)
	tr, err := translators.NewBrew(fr)
	if err != nil {
		t.Fatalf("NewBrew returned error: %v", err)
	}
	steps := []engine.Step{
		engine.NewPackageStep("a:foo", "foo", tr, "Foo utility"),
	}
	var buf bytes.Buffer
	if err := engine.PrintPlan(&buf, steps); err != nil {
		t.Fatalf("PrintPlan returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "brew install foo") {
		t.Errorf("output missing exact command:\n%s", out)
	}
	if !strings.Contains(out, "no changes made") {
		t.Errorf("output missing dry-run marker:\n%s", out)
	}
}
