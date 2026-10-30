package connector

import (
	"context"
	"encoding/json"
	"testing"

	"opskeeper/backend/engine"
	kclient "opskeeper/backend/mcpserver/kubernetes/client"
	kserver "opskeeper/backend/mcpserver/kubernetes/server"
)

func TestKubernetesDirectToolsMatchMCPToolCatalog(t *testing.T) {
	var direct map[string]string = map[string]string{}
	var schemas = map[string]map[string]any{}
	err := (&Service{}).resolveKubernetesTools(context.Background(), engine.ContextResource{}, func(name, description string, schemaJSON json.RawMessage, _ func(context.Context, map[string]any) (engine.ToolResult, error)) {
		direct[name] = description
		var schema map[string]any
		if err := json.Unmarshal(schemaJSON, &schema); err != nil {
			t.Fatalf("decode %s schema: %v", name, err)
		}
		schemas[name] = schema
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range kserver.AvailableTools() {
		if direct[tool.Name] != tool.Description {
			t.Errorf("tool %q direct description=%q MCP description=%q", tool.Name, direct[tool.Name], tool.Description)
		}
		delete(direct, tool.Name)
		properties := schemas[tool.Name]["properties"].(map[string]any)
		if _, ok := properties["context"]; !ok {
			t.Errorf("tool %q does not expose context", tool.Name)
		}
	}
	for name := range direct {
		t.Errorf("Kubernetes tool missing from MCP catalog %q", name)
	}
}

func TestKubernetesToolContextOverridesResourceDefault(t *testing.T) {
	base := kclient.ConnectionInput{Context: "resource-default"}
	if got := kubernetesToolConnection(base, map[string]any{"context": " alternate "}); got.Context != "alternate" {
		t.Fatalf("explicit context = %q, want alternate", got.Context)
	}
	if got := kubernetesToolConnection(base, map[string]any{}); got.Context != "resource-default" {
		t.Fatalf("default context = %q, want resource-default", got.Context)
	}
	if got := kubernetesToolConnection(base, map[string]any{"context": "  "}); got.Context != "resource-default" {
		t.Fatalf("blank context = %q, want resource-default", got.Context)
	}
}
