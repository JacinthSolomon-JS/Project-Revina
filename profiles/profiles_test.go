package profiles

import (
	"testing"

	"project-revina/internal/config"
)

// TestAllEmbeddedProfilesParse guards against profile/schema drift: every
// embedded profile must satisfy the strict loader.
func TestAllEmbeddedProfilesParse(t *testing.T) {
	names := Names()
	if len(names) == 0 {
		t.Fatal("no embedded profiles found")
	}
	for _, name := range names {
		data, err := Get(name)
		if err != nil {
			t.Fatalf("Get(%q): %v", name, err)
		}
		if _, err := config.Parse(data); err != nil {
			t.Errorf("profile %s does not parse: %v", name, err)
		}
	}
}
