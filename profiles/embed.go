// Package profiles embeds the default profile YAML files into the binary so a
// freshly built devsec works without a working directory on disk.
package profiles

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

//go:embed *.yaml
var files embed.FS

// Get returns embedded profile data by filename (e.g. "dev.yaml").
func Get(name string) ([]byte, error) {
	if name == "" {
		return nil, fmt.Errorf("profile name is empty")
	}
	data, err := files.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("embedded profile %q: %w", name, err)
	}
	return data, nil
}

// Names lists the embedded profile filenames in sorted order.
func Names() []string {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}
