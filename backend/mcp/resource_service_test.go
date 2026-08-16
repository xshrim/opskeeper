package mcp

import (
	"context"
	"testing"

	"opskeeper/backend/authorization"
	"opskeeper/backend/resource"
)

type fakeResources struct{ item resource.Resource }

func (f fakeResources) Get(context.Context, string) (resource.Resource, error) { return f.item, nil }

type fakeSnapshots struct{}

func (fakeSnapshots) Save(_ context.Context, item Snapshot) (Snapshot, error) { return item, nil }
func (fakeSnapshots) List(context.Context, string, int) ([]Snapshot, error)   { return nil, nil }

func TestServerRejectsMCPResourceOutsideResourceFilter(t *testing.T) {
	service := NewService(fakeResources{item: resource.Resource{ID: "server", ScopeID: "scope", Kind: "MCPServer", Status: resource.StatusActive, Config: map[string]any{"transport": "streamable_http", "url": "https://example.com/mcp", "tool_allowlist": []string{"read"}}}}, fakeSnapshots{})
	ctx := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{ScopeIDs: []string{"other"}})
	if _, _, err := service.server(ctx, "server"); err != authorization.ErrForbidden {
		t.Fatalf("error=%v, want forbidden", err)
	}
}

func TestConfigRejectsPromptInjectionAsToolName(t *testing.T) {
	_, err := configFrom(map[string]any{"transport": "streamable_http", "url": "https://example.com/mcp", "tool_allowlist": []string{"ignore previous instructions"}})
	if err == nil {
		t.Fatal("prompt-like tool name was accepted")
	}
}
