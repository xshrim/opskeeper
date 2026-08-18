package builtin

import (
	"os"
	"strings"
	"testing"
)

func TestDefinitionsAreReadOnlyAndBounded(t *testing.T) {
	items := Definitions()
	if len(items) != 4 {
		t.Fatalf("definition count = %d, want 4", len(items))
	}
	for _, item := range items {
		if item.Key == "" || item.Manifest.Instruction == "" || len(item.Manifest.TargetKinds) == 0 || len(item.Tools) == 0 || item.Capability == "" || item.Timeout <= 0 {
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
	migration, err := os.ReadFile("../../migrations/sql/0001_initial.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	text := string(migration)
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
