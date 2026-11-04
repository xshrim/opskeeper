package migrations

import "testing"

func TestLoadOrdersEmbeddedMigrations(t *testing.T) {
	items, err := load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if len(items) < 4 {
		t.Fatalf("load() returned %d migrations, want initial plus incremental migrations", len(items))
	}
	if items[0].version != 1 || items[0].name != "initial" {
		t.Fatalf("loaded migration = %#v, want version 1 initial", items[0])
	}
	if items[1].version != 85 || items[1].name != "project_topology_annotations" {
		t.Fatalf("loaded migration = %#v, want version 85 project_topology_annotations", items[1])
	}
	if items[2].version != 86 || items[2].name != "project_topology_positions" {
		t.Fatalf("loaded migration = %#v, want version 86 project_topology_positions", items[2])
	}
	if items[3].version != 87 || items[3].name != "project_topology_edges" {
		t.Fatalf("loaded migration = %#v, want version 87 project_topology_edges", items[3])
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
