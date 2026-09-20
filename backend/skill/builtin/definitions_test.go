package builtin

import (
	"os"
	"strings"
	"testing"
)

func TestDefinitionsAreReadOnlyAndBounded(t *testing.T) {
	items := Definitions()
	if len(items) != 9 {
		t.Fatalf("definition count = %d, want 9", len(items))
	}
	for _, item := range items {
		if item.Key == "" || item.Manifest.Instruction == "" || len(item.Tools) == 0 || item.Capability == "" || item.Timeout <= 0 {
			t.Fatalf("invalid definition %#v", item)
		}
		for _, tool := range item.Tools {
			if len(tool.InputSchema) == 0 {
				t.Fatalf("%s has tool without schema", item.Key)
			}
		}
	}
}

func TestDefinitionsMatchBuiltinMigrationNamesAndTools(t *testing.T) {
	entries, err := os.ReadDir("../../migrations/sql")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	var migrations strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") || strings.HasSuffix(entry.Name(), ".down.sql") {
			continue
		}
		migration, err := os.ReadFile("../../migrations/sql/" + entry.Name())
		if err != nil {
			t.Fatalf("read migration %s: %v", entry.Name(), err)
		}
		migrations.Write(migration)
	}
	text := migrations.String()
	for _, item := range Definitions() {
		if !strings.Contains(text, item.Manifest.Name) {
			t.Fatalf("migration is missing definition %q", item.Manifest.Name)
		}
		for _, tool := range item.Tools {
			if !strings.Contains(text, tool.Name) {
				t.Fatalf("migration is missing tool %q", tool.Name)
			}
		}
	}
}
