package migrations

import "testing"

func TestLoadOrdersEmbeddedMigrations(t *testing.T) {
	items, err := load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if len(items) != 5 {
		t.Fatalf("load() returned %d migrations, want five migrations", len(items))
	}
	if items[0].version != 1 || items[0].name != "initial" {
		t.Fatalf("loaded migration = %#v, want version 1 initial", items[0])
	}
	if items[1].version != 2 || items[1].name != "user_preferences" {
		t.Fatalf("loaded migration = %#v, want version 2 user_preferences", items[1])
	}
	if items[2].version != 3 || items[2].name != "one_time_password" {
		t.Fatalf("loaded migration = %#v, want version 3 one_time_password", items[2])
	}
	if items[3].version != 4 || items[3].name != "role_grant_hierarchy" {
		t.Fatalf("loaded migration = %#v, want version 4 role_grant_hierarchy", items[3])
	}
	if items[4].version != 5 || items[4].name != "administrator_role_permissions" {
		t.Fatalf("loaded migration = %#v, want version 5 administrator_role_permissions", items[4])
	}
	for _, item := range items {
		if item.sql == "" || item.downSQL == "" || item.checksum == "" {
			t.Fatalf("migration is missing up or down SQL: %#v", item)
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
