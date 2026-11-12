package engine

import (
	"encoding/json"
	"testing"
)

func TestDescribeToolParametersAddsMissingDescriptionsAndPreservesExistingOnes(t *testing.T) {
	raw := json.RawMessage(`{"type":"object","properties":{"namespace":{"type":"string"},"query":{"type":"string","description":"Use the exact server query syntax."}}}`)
	var schema struct {
		Properties map[string]map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(DescribeToolParameters("Prometheus", raw), &schema); err != nil {
		t.Fatal(err)
	}
	if schema.Properties["namespace"]["description"] == "" {
		t.Fatal("missing namespace parameter description")
	}
	if got := schema.Properties["query"]["description"]; got != "Use the exact server query syntax." {
		t.Fatalf("existing provider description was changed: %v", got)
	}
}
