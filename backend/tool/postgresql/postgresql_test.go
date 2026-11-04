package postgresql

import "testing"

func TestListToolsIsFixedReadOnlyCatalog(t *testing.T) {
	items := ListTools()
	if len(items) != 22 {
		t.Fatalf("tool count = %d, want 22", len(items))
	}
	for _, item := range items {
		if item.Name == "" || item.Description == "" {
			t.Fatalf("invalid tool %#v", item)
		}
	}
}

func TestDiagnosticSchemasRejectUnsafeInputs(t *testing.T) {
	if _, err := validateExplainQuery("UPDATE users SET name='x'"); err == nil {
		t.Fatal("write query unexpectedly accepted")
	}
	if _, err := validateExplainQuery("SELECT 1; SELECT 2"); err == nil {
		t.Fatal("multi-statement query unexpectedly accepted")
	}
	if _, err := validateExplainQuery("SELECT 1"); err != nil {
		t.Fatalf("read-only query rejected: %v", err)
	}
	if _, err := validateExplainQuery("WITH changed AS (DELETE FROM users RETURNING id) SELECT * FROM changed"); err == nil {
		t.Fatal("data-modifying CTE unexpectedly accepted")
	}
}

func TestTableColumnsInputHasNoSQLField(t *testing.T) {
	schema := InputSchema(nil)
	if _, ok := schema["query"]; ok {
		t.Fatal("connection schema unexpectedly exposes SQL query")
	}
}
