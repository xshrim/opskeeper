package resource

import "testing"

func TestValidateConfigRequiredAndPropertyTypes(t *testing.T) {
	schema := Schema{Kind: "Test", Schema: map[string]any{
		"type":                 "object",
		"required":             []any{"endpoint"},
		"additionalProperties": false,
		"properties": map[string]any{
			"endpoint": map[string]any{"type": "string"},
			"secure":   map[string]any{"type": "boolean"},
		},
	}}
	if err := validateConfig(map[string]any{"endpoint": "https://example.test", "secure": true}, schema); err != nil {
		t.Fatalf("validateConfig(valid) error = %v", err)
	}
	if err := validateConfig(map[string]any{}, schema); err == nil {
		t.Fatal("validateConfig(missing required) error = nil")
	}
	if err := validateConfig(map[string]any{"endpoint": 3}, schema); err == nil {
		t.Fatal("validateConfig(wrong type) error = nil")
	}
	if err := validateConfig(map[string]any{"endpoint": "ok", "unknown": true}, schema); err == nil {
		t.Fatal("validateConfig(unknown property) error = nil")
	}
}

func TestValidateConfigAllowsRegisteredPermissiveSchema(t *testing.T) {
	schema := Schema{Kind: "Redis", Schema: map[string]any{"type": "object", "additionalProperties": true}}
	if err := validateConfig(map[string]any{"address": "redis:6379", "database": 0}, schema); err != nil {
		t.Fatalf("validateConfig(permissive) error = %v", err)
	}
}
