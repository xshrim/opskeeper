package diagnosis

import (
	"encoding/json"
	"testing"
)

func TestSanitizeModelJSONRemovesInternalResourceJoinKeys(t *testing.T) {
	raw := json.RawMessage(`{
        "application_id": "app-internal",
        "target_resource_id": "resource-internal",
        "source_resource_ids": ["resource-source"],
        "selector": {"container_name": "orders-api"},
        "binding": {"database": "orders", "schema": "public"}
    }`)

	value, err := sanitizeModelJSON(raw)
	if err != nil {
		t.Fatalf("sanitizeModelJSON() error = %v", err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal sanitized value: %v", err)
	}
	if string(encoded) == "" {
		t.Fatal("sanitized value is empty")
	}
	for _, internal := range []string{"application_id", "target_resource_id", "source_resource_ids"} {
		if containsJSONKey(encoded, internal) {
			t.Fatalf("sanitized context still contains internal key %q: %s", internal, encoded)
		}
	}
	for _, external := range []string{"container_name", "database", "schema"} {
		if !containsJSONKey(encoded, external) {
			t.Fatalf("sanitized context removed external binding key %q: %s", external, encoded)
		}
	}
}

func containsJSONKey(raw []byte, key string) bool {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return false
	}
	return containsJSONKeyValue(value, key)
}

func containsJSONKeyValue(value any, key string) bool {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if containsJSONKeyValue(item, key) {
				return true
			}
		}
	case map[string]any:
		if _, ok := typed[key]; ok {
			return true
		}
		for _, item := range typed {
			if containsJSONKeyValue(item, key) {
				return true
			}
		}
	}
	return false
}
