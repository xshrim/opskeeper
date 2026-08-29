package aiengine

import "testing"

func TestRedactValueRemovesSensitiveFieldsRecursively(t *testing.T) {
	value := redactValue(map[string]any{
		"query":   "select 1",
		"api_key": "secret",
		"api-key": "secret",
		"nested":  map[string]any{"password": "hidden", "ok": true},
	})
	result := value.(map[string]any)
	if result["api_key"] != "[REDACTED]" || result["api-key"] != "[REDACTED]" || result["nested"].(map[string]any)["password"] != "[REDACTED]" || result["query"] != "select 1" {
		t.Fatalf("redacted value=%#v", result)
	}
}
