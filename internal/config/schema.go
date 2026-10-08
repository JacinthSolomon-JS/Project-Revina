package config

// Profile is the top-level declarative configuration. It is intentionally
// declarative: it describes intent ("install these packages", "apply this
// hardening") and never embeds shell snippets.
type Profile struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Tasks       []Task `yaml:"tasks"`
}

// Task is a single unit of intent inside a Profile.
type Task struct {
	ID          string   `yaml:"id"`
	Description string   `yaml:"description,omitempty"`
	Kind        TaskKind `yaml:"kind"`
	Packages    []string `yaml:"packages,omitempty"`
	Rules       []string `yaml:"rules,omitempty"`
}

// TaskKind identifies what a Task does. New kinds extend this enum; the planner
// must implement matching step construction.
type TaskKind string

const (
	KindPackage   TaskKind = "package"
	KindHardening TaskKind = "hardening"
)
