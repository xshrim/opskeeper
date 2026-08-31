package aiengine

import (
	"context"
	"errors"
	"testing"
	"time"

	"opskeeper/backend/authorization"
)

func TestPolicyGatewayEnforcesAuthorizationAndEmitsToolEvents(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register("resource-1", ToolFunc{Def: ToolDefinition{Name: "read", Source: "test", ReadOnly: true}, Fn: func(context.Context, map[string]any) (ToolResult, error) {
		return ToolResult{Output: map[string]any{"ok": true}, Untrusted: true}, nil
	}}); err != nil {
		t.Fatal(err)
	}
	var events []Event
	gateway := NewPolicyGateway(registry, func(_ context.Context, call ToolCall, definition ToolDefinition) error {
		if call.ScopeID != "scope-1" || !definition.ReadOnly {
			return errors.New("unexpected policy input")
		}
		return nil
	}, time.Second, 1024, 1)
	result, err := gateway.Invoke(context.Background(), ToolCall{ScopeID: "scope-1", ResourceID: "resource-1", Name: "read", Arguments: map[string]any{"query": "orders", "limit": 5}, EventSink: func(event Event) error {
		events = append(events, event)
		return nil
	}})
	if err != nil || result.Untrusted != true || len(events) != 3 {
		t.Fatalf("result=%+v err=%v events=%+v", result, err, events)
	}
	if events[0].Type != "tool.requested" || events[1].Type != "tool.started" || events[2].Type != "tool.completed" {
		t.Fatalf("unexpected events=%+v", events)
	}
	for _, event := range events {
		arguments, ok := event.Payload["arguments"].(map[string]any)
		if !ok || arguments["query"] != "orders" {
			t.Fatalf("tool event did not preserve arguments: %+v", event)
		}
		if limit := arguments["limit"]; limit != 5 && limit != float64(5) {
			t.Fatalf("tool event did not preserve numeric argument: %+v", event)
		}
	}
	output, ok := events[2].Payload["output"].(map[string]any)
	if !ok || output["ok"] != true {
		t.Fatalf("tool.completed did not include tool output: %+v", events[2])
	}
}

func TestPolicyGatewayAuditsProviderModelTimingAndErrors(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register("resource-1", ToolFunc{Def: ToolDefinition{Name: "read", Source: "test"}, Fn: func(context.Context, map[string]any) (ToolResult, error) {
		return ToolResult{Output: map[string]any{"password": "hidden"}}, nil
	}}); err != nil {
		t.Fatal(err)
	}
	audit := &memoryToolCallStore{}
	gateway := NewPolicyGateway(registry, nil, time.Second, 1024, 1)
	gateway.AuditStore = audit
	if _, err := gateway.Invoke(context.Background(), ToolCall{ExecutionID: "exec-1", Sequence: 2, ScopeID: "scope-1", ResourceID: "resource-1", Name: "read", ProviderResourceID: "provider-1", ModelName: "model-a", Arguments: map[string]any{"api-key": "secret"}}); err != nil {
		t.Fatal(err)
	}
	if len(audit.records) != 1 {
		t.Fatalf("audit records=%+v", audit.records)
	}
	record := audit.records[0]
	if record.ProviderResourceID != "provider-1" || record.ModelName != "model-a" || record.DurationMS < 0 || record.StartedAt.IsZero() || record.CompletedAt.IsZero() {
		t.Fatalf("audit timing/provider details=%+v", record)
	}
	if record.Arguments["api-key"] != "secret" {
		t.Fatalf("in-memory audit should retain source values before persistence: %+v", record.Arguments)
	}
}

func TestAuthorizeResourceUseUsesResourceAndScopeFilters(t *testing.T) {
	resource := ContextResource{ID: "resource-1", ScopeID: "scope-1"}
	allowed := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{ResourceIDs: []string{"resource-1"}})
	if err := AuthorizeResourceUse(allowed, resource); err != nil {
		t.Fatalf("allowed resource rejected: %v", err)
	}
	denied := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{ResourceIDs: []string{"resource-2"}})
	if !errors.Is(AuthorizeResourceUse(denied, resource), authorization.ErrForbidden) {
		t.Fatal("denied resource was accepted")
	}
	scopeDenied := authorization.WithScopeFilter(context.Background(), authorization.ScopeFilter{ScopeIDs: []string{"scope-2"}})
	if !errors.Is(AuthorizeResourceUse(scopeDenied, resource), authorization.ErrForbidden) {
		t.Fatal("denied scope was accepted")
	}
}

func TestNewContextToolingUsesResourceUsePolicy(t *testing.T) {
	tooling := NewContextTooling(fakeContextResourceReader{resources: map[string]ContextResource{
		"resource-1": {ID: "resource-1", ScopeID: "scope-1", Kind: "PostgreSQL", Status: "active"},
	}})
	if tooling.Registry == nil || tooling.Gateway == nil || tooling.Resolver.Registry != tooling.Registry {
		t.Fatal("context tooling was not composed with shared registry")
	}
	if err := tooling.Registry.Register("resource-1", ToolFunc{Def: ToolDefinition{Name: "read", Source: "test", ResourceID: "resource-1"}, Fn: func(context.Context, map[string]any) (ToolResult, error) {
		return ToolResult{Output: "ok"}, nil
	}}); err != nil {
		t.Fatal(err)
	}
	allowed := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{ResourceIDs: []string{"resource-1"}})
	if _, err := tooling.Gateway.Invoke(allowed, ToolCall{ScopeID: "scope-1", ResourceID: "resource-1", Name: "read"}); err != nil {
		t.Fatalf("allowed invocation failed: %v", err)
	}
	denied := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{ResourceIDs: []string{"resource-2"}})
	if _, err := tooling.Gateway.Invoke(denied, ToolCall{ScopeID: "scope-1", ResourceID: "resource-1", Name: "read"}); !errors.Is(err, ErrToolDenied) {
		t.Fatalf("denied invocation error=%v, want ErrToolDenied", err)
	}
}

func TestPolicyGatewayRejectsOversizedOutput(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register("resource-1", ToolFunc{Def: ToolDefinition{Name: "large", Source: "test"}, Fn: func(context.Context, map[string]any) (ToolResult, error) {
		return ToolResult{Output: "too large"}, nil
	}}); err != nil {
		t.Fatal(err)
	}
	gateway := NewPolicyGateway(registry, nil, time.Second, 2, 1)
	if _, err := gateway.Invoke(context.Background(), ToolCall{ResourceID: "resource-1", Name: "large"}); !errors.Is(err, ErrToolLimited) {
		t.Fatalf("error=%v, want ErrToolLimited", err)
	}
}

func TestToolRegistryRejectsEmptyResourceAndSupportsRefresh(t *testing.T) {
	registry := NewToolRegistry()
	tool := ToolFunc{Def: ToolDefinition{Name: "read", Source: "test"}, Fn: func(context.Context, map[string]any) (ToolResult, error) {
		return ToolResult{Output: "v1"}, nil
	}}
	if err := registry.Register("", tool); !errors.Is(err, ErrToolInvalid) {
		t.Fatalf("empty resource registration error=%v, want ErrToolInvalid", err)
	}
	if err := registry.Register("resource-1", tool); err != nil {
		t.Fatal(err)
	}
	refreshed := ToolFunc{Def: ToolDefinition{Name: "read", Source: "test", Description: "refreshed"}, Fn: tool.Fn}
	if err := registry.Upsert("resource-1", refreshed); err != nil {
		t.Fatal(err)
	}
	if got := registry.List("resource-1"); len(got) != 1 || got[0].Description != "refreshed" {
		t.Fatalf("registry after refresh=%+v", got)
	}
}

type fakeContextResourceReader struct{ resources map[string]ContextResource }

type memoryToolCallStore struct{ records []ToolCallRecord }

func (m *memoryToolCallStore) RecordToolCall(_ context.Context, record ToolCallRecord) error {
	m.records = append(m.records, record)
	return nil
}
func (m *memoryToolCallStore) ListToolCalls(_ context.Context, _ string, _ int) ([]ToolCallRecord, error) {
	return m.records, nil
}

func (f fakeContextResourceReader) Get(_ context.Context, id string) (ContextResource, error) {
	resource, ok := f.resources[id]
	if !ok {
		return ContextResource{}, errors.New("resource not found")
	}
	return resource, nil
}

type fakeContextProvider struct{}

func (fakeContextProvider) Kinds() []string { return []string{"PostgreSQL"} }
func (fakeContextProvider) Resolve(_ context.Context, resource ContextResource) ([]Tool, []ContextFact, error) {
	return []Tool{ToolFunc{Def: ToolDefinition{Name: "postgres.inspect", Source: "connector", ResourceID: resource.ID}}}, []ContextFact{{ResourceID: resource.ID, Kind: resource.Kind, Untrusted: true}}, nil
}

func TestResourceContextResolverDeduplicatesAndAuthorizesResources(t *testing.T) {
	registry := NewToolRegistry()
	resolver := ResourceContextResolver{
		Resources: fakeContextResourceReader{resources: map[string]ContextResource{"resource-1": {ID: "resource-1", ScopeID: "scope-1", Kind: "PostgreSQL", Status: "active"}}},
		Providers: []ContextProvider{fakeContextProvider{}},
		Registry:  registry,
		Authorize: func(_ context.Context, resource ContextResource) error {
			if resource.ScopeID != "scope-1" {
				return errors.New("forbidden")
			}
			return nil
		},
	}
	resolved, err := resolver.Resolve(context.Background(), ContextRequest{ResourceIDs: []string{"resource-1", "resource-1"}})
	if err != nil || len(resolved.Resources) != 1 || len(resolved.Tools) != 1 || len(resolved.Facts) != 1 {
		t.Fatalf("resolved=%+v err=%v", resolved, err)
	}
	if _, ok := registry.Get("resource-1", "postgres.inspect"); !ok {
		t.Fatal("resolved tool was not registered")
	}
	if _, err := resolver.Resolve(context.Background(), ContextRequest{ResourceIDs: []string{"resource-1"}}); err != nil {
		t.Fatalf("re-resolving a shared resource should refresh tools: %v", err)
	}
}

func TestResourceContextResolverRejectsInactiveResource(t *testing.T) {
	resolver := ResourceContextResolver{Resources: fakeContextResourceReader{resources: map[string]ContextResource{"resource-1": {ID: "resource-1", Kind: "PostgreSQL", Status: "disabled"}}}}
	if _, err := resolver.Resolve(context.Background(), ContextRequest{ResourceIDs: []string{"resource-1"}}); err == nil {
		t.Fatal("inactive resource was accepted")
	}
}
