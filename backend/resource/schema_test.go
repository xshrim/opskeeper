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

func TestValidateConfigEnforcesNestedSchemaConstraints(t *testing.T) {
	schema := Schema{Kind: "Nested", Schema: map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type":    "object", "additionalProperties": false,
		"required": []any{"models"},
		"properties": map[string]any{
			"models": map[string]any{
				"type": "array", "minItems": 1,
				"items": map[string]any{
					"type": "object", "additionalProperties": false,
					"required":   []any{"name"},
					"properties": map[string]any{"name": map[string]any{"type": "string", "minLength": 1}},
				},
			},
		},
	}}
	if err := validateConfig(map[string]any{"models": []any{map[string]any{"name": "gpt"}}}, schema); err != nil {
		t.Fatalf("validateConfig(valid nested) error = %v", err)
	}
	if err := validateConfig(map[string]any{"models": []any{map[string]any{}}}, schema); err == nil {
		t.Fatal("validateConfig(missing nested required) error = nil")
	}
	if err := validateConfig(map[string]any{"models": []any{}}, schema); err == nil {
		t.Fatal("validateConfig(nested minItems) error = nil")
	}
}

func TestResourceSubtypeDirectAgentKinds(t *testing.T) {
	if got := normalizeResourceSubtype("Kafka", ""); got != "Direct" {
		t.Fatalf("normalizeResourceSubtype(Kafka, empty) = %q, want Direct", got)
	}
	for _, subtype := range []string{"Direct", "Agent"} {
		if err := validateResourceSubtype("Host", subtype); err != nil {
			t.Fatalf("validateResourceSubtype(Host, %q) error = %v", subtype, err)
		}
	}
	if err := validateResourceSubtype("PostgreSQL", "API"); err == nil {
		t.Fatal("validateResourceSubtype(PostgreSQL, API) error = nil")
	}
	if err := validateResourceSubtype("Application", "虚拟机"); err != nil {
		t.Fatalf("validateResourceSubtype(Application) error = %v", err)
	}
}
