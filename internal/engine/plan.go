package engine

import (
	"fmt"

	"project-revina/internal/config"
)

// RuleFactory converts a logical hardening rule name into a Step for the host
// OS. A rule that is recognized but not implemented on this OS returns a nil
// Step (skip); an unknown rule returns an error. It exists here so the planner
// stays OS-agnostic while cmd wires in the hardening package.
type RuleFactory func(rule string) (Step, error)

// BuildPlan converts a validated profile into an ordered list of Steps. It is
// purely constructive: nothing is checked or applied here, so calling it never
// mutates or inspects the host. The Translator and RuleFactory are supplied by
// the caller so this package stays free of translator- and OS-specific
// knowledge.
func BuildPlan(p *config.Profile, tr Translator, rules RuleFactory) ([]Step, error) {
	steps := make([]Step, 0, len(p.Tasks))
	for _, task := range p.Tasks {
		switch task.Kind {
		case config.KindPackage:
			for _, pkg := range task.Packages {
				steps = append(steps, NewPackageStep(task.ID+":"+pkg, pkg, tr, task.Description))
			}
		case config.KindHardening:
			if rules == nil {
				return nil, fmt.Errorf("task %q: hardening requires a rule factory", task.ID)
			}
			for _, rule := range task.Rules {
				step, err := rules(rule)
				if err != nil {
					return nil, fmt.Errorf("task %q rule %q: %w", task.ID, rule, err)
				}
				if step != nil {
					steps = append(steps, step)
				}
			}
		default:
			return nil, fmt.Errorf("task %q: unsupported kind %q", task.ID, task.Kind)
		}
	}
	return steps, nil
}
