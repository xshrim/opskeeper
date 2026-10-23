package migrations

import "testing"

func TestLoadOrdersEmbeddedMigrations(t *testing.T) {
	items, err := load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("load() returned %d migrations, want baseline, AI-name, and Persona-tag migrations", len(items))
	}
	if items[0].version != 1 || items[0].name != "initial" {
		t.Fatalf("loaded migration = %#v, want version 1 initial", items[0])
	}
	if items[1].version != 2 || items[1].name != "unique_ai_names" {
		t.Fatalf("loaded migration = %#v, want version 2 unique_ai_names", items[1])
	}
	if items[2].version != 3 || items[2].name != "persona_tags" {
		t.Fatalf("loaded migration = %#v, want version 3 persona_tags", items[2])
	}
	for _, item := range items {
		if item.sql == "" || item.checksum == "" {
			t.Fatalf("migration is missing SQL or checksum: %#v", item)
		}
		if item.checksum != migrationChecksum([]byte(item.sql)) {
			t.Fatalf("migration checksum does not match SQL: %#v", item)
		}
	}
	for index := 1; index < len(items); index++ {
		if items[index-1].version >= items[index].version {
			t.Fatalf("migrations are not ordered: %#v", items)
		}
	}
}
