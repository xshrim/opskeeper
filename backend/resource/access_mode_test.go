package resource

import (
	"context"
	"fmt"
	"testing"
)

func TestNormalizeAccessModeDefaultsConnectionResourcesToDirect(t *testing.T) {
	mode, err := normalizeAccessMode("Docker", "")
	if err != nil || mode != AccessModeDirect {
		t.Fatalf("mode=%q err=%v", mode, err)
	}
	if mode, err := normalizeAccessMode("Application", ""); err != nil || mode != "" {
		t.Fatalf("mode=%q err=%v", mode, err)
	}
}

func TestNormalizeAccessModeLeavesMCPServerSubtypeAlone(t *testing.T) {
	for _, subtype := range []string{"StreamHTTP", "SSE"} {
		mode, err := normalizeAccessMode("MCPServer", subtype)
		if err != nil || mode != "" {
			t.Fatalf("normalizeAccessMode(MCPServer, %q) = %q, %v", subtype, mode, err)
		}
	}
}

type accessModeStore struct {
	resources map[string]Resource
	scopeType string
}

func (s *accessModeStore) ScopeType(context.Context, string) (string, error) {
	if s.scopeType != "" {
		return s.scopeType, nil
	}
	return "project", nil
}

func (s *accessModeStore) Create(_ context.Context, input CreateInput) (Resource, error) {
	item := Resource{ID: "resource-1", ScopeID: input.ScopeID, Kind: input.Kind, Name: input.Name, Subtype: input.Subtype, AgentRef: input.AgentRef, CredentialID: input.CredentialID, Status: input.Status, Config: input.Config, Labels: input.Labels}
	if item.Status == "" {
		item.Status = StatusActive
	}
	s.resources[item.ID] = item
	return item, nil
}
func (s *accessModeStore) UpsertImported(c context.Context, i ImportedInput) (Resource, error) {
	return s.Create(c, i)
}
func (s *accessModeStore) List(context.Context, Pagination, string, map[string]string) (Page[Resource], error) {
	return Page[Resource]{}, nil
}
func (s *accessModeStore) Get(_ context.Context, id string) (Resource, error) {
	item, ok := s.resources[id]
	if !ok {
		return Resource{}, ErrNotFound
	}
	return item, nil
}
func (s *accessModeStore) Update(_ context.Context, id string, input UpdateInput) (Resource, error) {
	item, e := s.Get(context.Background(), id)
	if e != nil {
		return Resource{}, e
	}
	if input.Subtype != nil {
		item.Subtype = *input.Subtype
	}
	if input.AgentRef != nil {
		item.AgentRef = *input.AgentRef
	}
	if input.CredentialID != nil {
		item.CredentialID = *input.CredentialID
	}
	s.resources[id] = item
	return item, nil
}
func (s *accessModeStore) Delete(context.Context, string) error { return nil }
func (s *accessModeStore) GetSchema(context.Context, string, int) (Schema, error) {
	return Schema{Kind: "Docker", Version: 1, Schema: map[string]any{"type": "object", "additionalProperties": true}}, nil
}
func (s *accessModeStore) ListSchemas(context.Context) ([]Schema, error) { return nil, nil }
func (s *accessModeStore) CreateRelation(context.Context, CreateRelationInput, string) (Relation, error) {
	return Relation{}, nil
}
func (s *accessModeStore) ListRelations(context.Context, string) ([]Relation, error) { return nil, nil }
func (s *accessModeStore) DeleteRelation(context.Context, string, string) error      { return nil }
func (s *accessModeStore) Topology(context.Context, string, int, int) ([]TopologyNode, error) {
	return nil, nil
}
func (s *accessModeStore) SetDefault(context.Context, string, string, string) (Default, error) {
	return Default{}, nil
}
func (s *accessModeStore) ResolveDefault(context.Context, string, string) (Resource, error) {
	return Resource{}, ErrNotFound
}

func TestServiceCreateAgentRequiresAndStoresAssociation(t *testing.T) {
	serverID := "mcp-server"
	store := &accessModeStore{resources: map[string]Resource{serverID: {ID: serverID, ScopeID: "scope-1", Kind: "MCPServer", Status: StatusActive}}}
	item, err := NewService(store).Create(context.Background(), CreateInput{ScopeID: "scope-1", Kind: "Docker", Subtype: "Agent", AgentRef: &serverID, Name: "agent", Config: map[string]any{}})
	if err != nil || item.Subtype != "Agent" || item.AgentRef == nil || *item.AgentRef != serverID {
		t.Fatalf("item=%+v err=%v", item, err)
	}
}

func TestApplicationRequiresProjectAndUniqueInstanceLocator(t *testing.T) {
	store := &accessModeStore{resources: map[string]Resource{
		"host-1":       {ID: "host-1", ScopeID: "team-scope", Kind: "Host", Status: StatusActive},
		"docker-1":     {ID: "docker-1", ScopeID: "project-scope", Kind: "Docker", Status: StatusActive},
		"kube-1":       {ID: "kube-1", ScopeID: "project-scope", Kind: "Kubernetes", Status: StatusActive},
		"agent-host":   {ID: "agent-host", ScopeID: "project-scope", Kind: "Host", Subtype: AccessModeAgent, Status: StatusActive},
		"agent-docker": {ID: "agent-docker", ScopeID: "project-scope", Kind: "Docker", Subtype: AccessModeAgent, Status: StatusActive},
		"agent-kube":   {ID: "agent-kube", ScopeID: "project-scope", Kind: "Kubernetes", Subtype: AccessModeAgent, Status: StatusActive},
		"agent-loki":   {ID: "agent-loki", ScopeID: "project-scope", Kind: "Loki", Subtype: AccessModeAgent, Status: StatusActive},
	}}
	service := NewService(store)
	config := map[string]any{
		"access_mode": "virtual_machine",
		"instances": []any{map[string]any{
			"host_resource_id": "host-1",
			"process_keyword":  "orders&java",
			"log_source":       map[string]any{"type": "path", "path": "/var/log/orders.log"},
		}},
	}
	if _, err := service.Create(context.Background(), CreateInput{ScopeID: "project-scope", Kind: "Application", Subtype: "virtual_machine", Name: "orders", Config: config}); err != nil {
		t.Fatalf("Create(Application) error = %v", err)
	}
	store.scopeType = "team"
	if _, err := service.Create(context.Background(), CreateInput{ScopeID: "team-scope", Kind: "Application", Subtype: "virtual_machine", Name: "orders-team", Config: config}); err == nil {
		t.Fatal("Create(Application) on team scope succeeded")
	}
	store.scopeType = "project"
	duplicate := map[string]any{
		"access_mode": "virtual_machine",
		"instances": []any{
			map[string]any{"host_resource_id": "host-1", "process_keyword": "orders", "log_source": map[string]any{"type": "path", "path": "/var/log/orders.log"}},
			map[string]any{"host_resource_id": "host-1", "process_keyword": "orders", "log_source": map[string]any{"type": "path", "path": "/var/log/orders.log"}},
		},
	}
	if _, err := service.Create(context.Background(), CreateInput{ScopeID: "project-scope", Kind: "Application", Name: "duplicate", Config: duplicate}); err == nil {
		t.Fatal("Create(Application) with duplicate host association succeeded")
	}
	invalidModes := []map[string]any{
		{"access_mode": "", "instances": []any{}},
		{"access_mode": "virtual_machine", "instances": []any{map[string]any{"host_resource_id": "host-1", "log_source": map[string]any{"type": "path", "path": "/tmp/app.log"}}}},
		{"access_mode": "containerized", "instances": []any{map[string]any{"docker_resource_id": "docker-1"}}},
		{"access_mode": "cloud_native", "instances": []any{map[string]any{"kubernetes_resource_id": "kube-1", "namespace": "default", "workload_name": "orders"}}},
	}
	for index, invalidConfig := range invalidModes {
		if _, err := service.Create(context.Background(), CreateInput{ScopeID: "project-scope", Kind: "Application", Name: fmt.Sprintf("invalid-%d", index), Config: invalidConfig}); err == nil {
			t.Fatalf("Create(Application) invalid case %d succeeded", index)
		}
	}
	for index, agentConfig := range []map[string]any{
		{"access_mode": "virtual_machine", "instances": []any{map[string]any{"host_resource_id": "agent-host", "process_keyword": "orders", "log_source": map[string]any{"type": "path", "path": "/tmp/app.log"}}}},
		{"access_mode": "containerized", "instances": []any{map[string]any{"docker_resource_id": "agent-docker", "container_name": "orders", "log_source": map[string]any{"type": "stdout"}}}},
		{"access_mode": "cloud_native", "instances": []any{map[string]any{"kubernetes_resource_id": "agent-kube", "namespace": "orders", "workload_kind": "Deployment", "workload_name": "orders", "log_source": map[string]any{"type": "stdout"}}}},
		{"access_mode": "virtual_machine", "instances": []any{map[string]any{"host_resource_id": "agent-host", "process_keyword": "orders", "log_source": map[string]any{"type": "query", "resource_id": "agent-loki", "query": "{app=\"orders\"}"}}}},
	} {
		if _, err := service.Create(context.Background(), CreateInput{ScopeID: "project-scope", Kind: "Application", Name: fmt.Sprintf("agent-%d", index), Config: agentConfig}); err != nil {
			t.Fatalf("Create(Application) agent case %d error = %v", index, err)
		}
	}
}
