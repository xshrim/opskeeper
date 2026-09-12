package postgresql

import "testing"

func TestListToolsIsFixedReadOnlyCatalog(t *testing.T) {
	items := ListTools()
	if len(items) != 12 {
		t.Fatalf("tool count = %d, want 12", len(items))
	}
	for _, item := range items {
		if item.Name == "" || item.Description == "" {
			t.Fatalf("invalid tool %#v", item)
		}
	}
}

func TestTableColumnsInputHasNoSQLField(t *testing.T) {
	schema := InputSchema(nil)
	if _, ok := schema["query"]; ok {
		t.Fatal("connection schema unexpectedly exposes SQL query")
	}
}
