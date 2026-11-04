package mysql

import "testing"

func TestFixedToolCatalogIncludesRelationalDiagnostics(t *testing.T) {
	want := map[string]bool{"mysql_slow_queries": false, "mysql_active_queries": false, "mysql_lock_waits": false, "mysql_connection_stats": false, "mysql_table_structure": false}
	for _, tool := range ListTools() {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("tool catalog missing %s", name)
		}
	}
}
