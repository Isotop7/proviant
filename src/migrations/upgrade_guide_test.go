package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// upgradeGuidePath is the operator guide that names the migrations.
const upgradeGuidePath = "../../docs/upgrade.md"

// TestUpgradeGuideNamesLiveMigrations fails when the upgrade guide names a
// migration the code no longer has. A guide that points at a migration that
// was removed sends the operator looking for a step that is not there.
func TestUpgradeGuideNamesLiveMigrations(t *testing.T) {
	content, err := os.ReadFile(upgradeGuidePath)
	if err != nil {
		t.Fatalf("reading %s failed: %v", upgradeGuidePath, err)
	}

	names := migrationNamesInGuide(string(content))
	if len(names) == 0 {
		t.Fatalf("%s names no migration", upgradeGuidePath)
	}
	for _, name := range names {
		if !migrationExists(name) {
			t.Errorf("%s names %q, which is no migration of this package", upgradeGuidePath, name)
		}
	}
}

// migrationNamesInGuide returns the migration names the guide names in a table
// row. A row starts with the name in backticks.
func migrationNamesInGuide(guide string) []string {
	var names []string
	for line := range strings.SplitSeq(guide, "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		name, _, found := strings.Cut(strings.TrimPrefix(line, "| `"), "`")
		if !found {
			continue
		}
		names = append(names, name)
	}
	return names
}

// migrationExists reports whether the package holds a migration with the name.
func migrationExists(name string) bool {
	entries, err := os.ReadDir(".")
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(".", entry.Name()))
		if readErr != nil {
			continue
		}
		if strings.Contains(string(content), "func "+name+"(") {
			return true
		}
	}
	return false
}
