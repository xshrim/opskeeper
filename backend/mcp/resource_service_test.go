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

func TestConfigAcceptsNamespacedMCPToolName(t *testing.T) {
	if _, err := configFrom(map[string]any{"transport": "streamable_http", "url": "http://127.0.0.1:3100/mcp", "tool_allowlist": []string{"docker:list_containers"}}); err != nil {
		t.Fatalf("namespaced MCP tool name rejected: %v", err)
	}
}

func TestConfigAcceptsSSEAndDefaults(t *testing.T) {
	config, err := configFrom(map[string]any{"transport": "sse", "url": "http://127.0.0.1:3100/sse"})
	if err != nil {
		t.Fatalf("SSE config rejected: %v", err)
	}
	if config.TimeoutSeconds != 120 || config.MaxResponseBytes != defaultMaxResponseBytes {
		t.Fatalf("defaults = timeout %d, max %d", config.TimeoutSeconds, config.MaxResponseBytes)
	}
}

func TestConfigAcceptsWildcardAllowlist(t *testing.T) {
	config, err := configFrom(map[string]any{"transport": "streamable_http", "url": "http://127.0.0.1:3100/mcp", "tool_allowlist": []string{"docker:*", "alerts.?"}})
	if err != nil || !matchesTool("docker:list", config.ToolAllowlist) || matchesTool("alerts.list", config.ToolAllowlist) {
		t.Fatalf("wildcard config/result invalid: config=%#v err=%v", config, err)
	}
}

func TestConfigRejectsHeaderInjection(t *testing.T) {
	if _, err := configFrom(map[string]any{"transport": "sse", "url": "http://127.0.0.1:3100/sse", "request_headers": map[string]any{"X-Test": "ok\r\nInjected: yes"}}); err == nil {
		t.Fatal("header injection was accepted")
	}
}

func TestConfigHonorsEnhancedSecurityURLPolicy(t *testing.T) {
	input := map[string]any{"transport": "streamable_http", "url": "http://127.0.0.1:3100/mcp"}
	if _, err := configFromWithSecurity(input, true); err == nil {
		t.Fatal("HTTP endpoint accepted with enhanced security")
	}
	if _, err := configFromWithSecurity(input, false); err != nil {
		t.Fatalf("HTTP endpoint rejected with enhanced security disabled: %v", err)
	}
}
