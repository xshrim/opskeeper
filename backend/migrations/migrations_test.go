package migrations

import (
	"strings"
	"testing"
)

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

func TestRepositoryApplicationTargetsDropPrimaryKeyBeforeMakingResourceNullable(t *testing.T) {
	items, err := load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	for _, item := range items {
		if item.version != 90 {
			continue
		}
		dropPrimaryKey := strings.Index(item.sql, "DROP CONSTRAINT inspection_policy_targets_pkey")
		makeResourceNullable := strings.Index(item.sql, "ALTER COLUMN resource_id DROP NOT NULL")
		if dropPrimaryKey < 0 || makeResourceNullable < 0 || dropPrimaryKey > makeResourceNullable {
			t.Fatalf("migration 90 must drop the resource_id primary key before making it nullable")
		}
		return
	}
	t.Fatal("migration 90 was not loaded")
}

func TestResourceTypeLabelsMigrationCoversRequestedKinds(t *testing.T) {
	items, err := load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	for _, item := range items {
		if item.version == 91 {
			for _, expected := range []string{"'Artifact' THEN '制品仓库'", "'Repository' THEN '代码仓库'", "'Nacos' THEN '注册中心和配置中心'", "'TongHttpServer' THEN '前端代理'", "'Kafka' THEN '消息队列'", "'Elasticsearch' THEN '文档检索'", "'MinIO' THEN '对象存储'", "'MCPServer' THEN '工具接入'", "'Monitor' THEN '可观察性'"} {
				if !strings.Contains(item.sql, expected) {
					t.Errorf("migration 91 does not contain %q", expected)
				}
			}
			return
		}
	}
	t.Fatal("migration 91 was not loaded")
}
