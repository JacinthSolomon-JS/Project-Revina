package mapping

import (
	"embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed tables/*.yaml
var tablesFS embed.FS

// Table maps logical package names to a native package manager's names. A
// logical name with no entry resolves to itself (identity mapping).
type Table struct {
	manager string
	entries map[string]string
}

// Load reads the YAML table for a package manager name (e.g. "brew", "apt").
func Load(manager string) (*Table, error) {
	data, err := tablesFS.ReadFile("tables/" + manager + ".yaml")
	if err != nil {
		return nil, fmt.Errorf("mapping table %q: %w", manager, err)
	}
	var entries map[string]string
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("mapping table %q: %w", manager, err)
	}
	return &Table{manager: manager, entries: entries}, nil
}

// Manager returns the manager name this table targets.
func (t *Table) Manager() string { return t.manager }

// Resolve returns the native package name for a logical name. When no mapping
// exists the trimmed logical name is returned unchanged. Mappings are added to
// the YAML tables, never to Go code.
func (t *Table) Resolve(logical string) string {
	name := strings.TrimSpace(logical)
	if native, ok := t.entries[name]; ok && native != "" {
		return native
	}
	return name
}
