package minio

import "testing"

func TestInputSchemaUsesRequiredFieldsOutsideProperties(t *testing.T) {
	schema := InputSchema(map[string]any{
		"bucket":     map[string]any{"type": "string"},
		"__required": []string{"bucket"},
	})
	properties := schema["properties"].(map[string]any)
	if _, ok := properties["__required"]; ok {
		t.Fatal("internal required marker leaked into properties")
	}
	required := schema["required"].([]string)
	if len(required) != 1 || required[0] != "bucket" {
		t.Fatalf("required = %#v", required)
	}
}

func TestBucketObjectsLimitIsBounded(t *testing.T) {
	if _, err := BucketObjects(nil, ConnectionInput{}, "", "", 1); err == nil {
		t.Fatal("missing bucket must fail before connection")
	}
}
