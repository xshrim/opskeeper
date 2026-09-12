package connector

import (
	"context"
	"encoding/json"
	"testing"

	"opskeeper/backend/aiengine"
	kserver "opskeeper/backend/mcpserver/kubernetes/server"
)

func TestKubernetesDirectToolsMatchMCPToolCatalog(t *testing.T) {
	var direct map[string]string = map[string]string{}
	err := (&Service{}).resolveKubernetesTools(context.Background(), aiengine.ContextResource{}, func(name, description string, _ json.RawMessage, _ func(context.Context, map[string]any) (aiengine.ToolResult, error)) {
		direct[name] = description
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range kserver.AvailableTools() {
		if direct[tool.Name] != tool.Description {
			t.Errorf("tool %q direct description=%q MCP description=%q", tool.Name, direct[tool.Name], tool.Description)
		}
		delete(direct, tool.Name)
	}
	for name := range direct {
		t.Errorf("Kubernetes tool missing from MCP catalog %q", name)
	}
}
