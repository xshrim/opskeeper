package resource

import (
	"context"
	"strings"
	"testing"
)

func TestNormalizeAccessModeDefaultsConnectionResourcesToDirect(t *testing.T) {
	mode, err := normalizeAccessMode("Docker", "", "")
	if err != nil || mode != AccessModeDirect {
		t.Fatalf("mode=%q err=%v, want direct", mode, err)
	}
	if mode, err := normalizeAccessMode("Application", "", ""); err != nil || mode != "" {
		t.Fatalf("non-connection mode=%q err=%v, want empty", mode, err)
	}
}

func TestNormalizeAccessModeRejectsConflictingFields(t *testing.T) {
	_, err := normalizeAccessMode("Docker", AccessModeDirect, "Agent")
	if err == nil || !strings.Contains(err.Error(), "must match") {
		t.Fatalf("error=%v, want access mode/subtype conflict", err)
	}
}

func TestNormalizeAccessModeRejectsUnsupportedExplicitMode(t *testing.T) {
	_, err := normalizeAccessMode("Application", AccessModeAgent, "")
	if err == nil || !strings.Contains(err.Error(), "does not support") {
		t.Fatalf("error=%v, want unsupported mode", err)
	}
}

type accessModeStore struct {
	resources map[string]Resource
}

func (s *accessModeStore) Create(_ context.Context, input CreateInput) (Resource, error) {
	item := Resource{ID: "resource-1", ScopeID: input.ScopeID, Kind: input.Kind, Name: input.Name, Subtype: input.Subtype, AccessMode: input.AccessMode, MCPServerResourceID: input.MCPServerResourceID, Status: input.Status, Config: input.Config, Labels: input.Labels}
	if item.Status == "" {
		item.Status = StatusActive
	}
	if s.resources == nil {
		s.resources = map[string]Resource{}
	}
	s.resources[item.ID] = item
	return item, nil
}

func (s *accessModeStore) UpsertImported(ctx context.Context, input ImportedInput) (Resource, error) {
	return s.Create(ctx, input)
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
	item, err := s.Get(context.Background(), id)
	if err != nil {
		return Resource{}, err
	}
	if input.Subtype != nil {
		item.Subtype = *input.Subtype
	}
	if input.AccessMode != nil {
		item.AccessMode = *input.AccessMode
	}
	if input.MCPServerResourceID != nil {
		item.MCPServerResourceID = *input.MCPServerResourceID
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

func TestServiceCreateAgentRequiresAndStoresMCPAssociation(t *testing.T) {
	serverID := "mcp-server"
	store := &accessModeStore{resources: map[string]Resource{serverID: {ID: serverID, ScopeID: "scope-1", Kind: "MCPServer", Status: StatusActive}}}
	service := NewService(store)
	item, err := service.Create(context.Background(), CreateInput{ScopeID: "scope-1", Kind: "Docker", Name: "agent", AccessMode: AccessModeAgent, MCPServerResourceID: &serverID, Config: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if item.AccessMode != AccessModeAgent || item.Subtype != "Agent" || item.MCPServerResourceID == nil || *item.MCPServerResourceID != serverID {
		t.Fatalf("created resource=%+v, want Agent association", item)
	}
}

func TestServiceUpdateAgentToDirectClearsMCPAssociation(t *testing.T) {
	serverID := "mcp-server"
	store := &accessModeStore{resources: map[string]Resource{
		"logical": {ID: "logical", ScopeID: "scope-1", Kind: "Docker", Name: "agent", Subtype: "Agent", AccessMode: AccessModeAgent, MCPServerResourceID: &serverID, Status: StatusActive},
	}}
	service := NewService(store)
	mode := AccessModeDirect
	item, err := service.Update(context.Background(), "logical", UpdateInput{AccessMode: &mode})
	if err != nil {
		t.Fatal(err)
	}
	if item.AccessMode != AccessModeDirect || item.Subtype != "Direct" || item.MCPServerResourceID != nil {
		t.Fatalf("updated resource=%+v, want direct without MCP association", item)
	}
}
