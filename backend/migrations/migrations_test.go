package migrations

import "testing"

func TestLoadOrdersEmbeddedMigrations(t *testing.T) {
	items, err := load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if len(items) != 15 {
		t.Fatalf("load() returned %d migrations, want fifteen migrations", len(items))
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
	if items[5].version != 6 || items[5].name != "project_viewer_resource_grants" {
		t.Fatalf("loaded migration = %#v, want version 6 project_viewer_resource_grants", items[5])
	}
	if items[6].version != 7 || items[6].name != "remove_skill_permissions" {
		t.Fatalf("loaded migration = %#v, want version 7 remove_skill_permissions", items[6])
	}
	if items[7].version != 8 || items[7].name != "resource_subtype" {
		t.Fatalf("loaded migration = %#v, want version 8 resource_subtype", items[7])
	}
	if items[8].version != 9 || items[8].name != "ai_engine_schema" {
		t.Fatalf("loaded migration = %#v, want version 9 ai_engine_schema", items[8])
	}
	if items[9].version != 10 || items[9].name != "ai_engine_unified_schema" {
		t.Fatalf("loaded migration = %#v, want version 10 ai_engine_unified_schema", items[9])
	}
	if items[10].version != 11 || items[10].name != "ai_engine_defaults" {
		t.Fatalf("loaded migration = %#v, want version 11 ai_engine_defaults", items[10])
	}
	if items[11].version != 12 || items[11].name != "ai_engine_config_fields" {
		t.Fatalf("loaded migration = %#v, want version 12 ai_engine_config_fields", items[11])
	}
	if items[12].version != 13 || items[12].name != "rename_ai_engine_manage_permission" {
		t.Fatalf("loaded migration = %#v, want version 13 rename_ai_engine_manage_permission", items[12])
	}
	if items[13].version != 14 || items[13].name != "rename_ai_model" {
		t.Fatalf("loaded migration = %#v, want version 14 rename_ai_model", items[13])
	}
	if items[14].version != 15 || items[14].name != "ai_model_schema_labels" {
		t.Fatalf("loaded migration = %#v, want version 15 ai_model_schema_labels", items[14])
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
