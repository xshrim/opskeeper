package migrations

import "testing"

func TestLoadOrdersEmbeddedMigrations(t *testing.T) {
	items, err := load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if len(items) == 0 {
		t.Fatal("load() returned no migrations")
	}
	for _, item := range items {
		if item.sql == "" || item.downSQL == "" {
			t.Fatalf("migration is missing up or down SQL: %#v", item)
		}
	}
	for index := 1; index < len(items); index++ {
		if items[index-1].version >= items[index].version {
			t.Fatalf("migrations are not ordered: %#v", items)
		}
	}
}
