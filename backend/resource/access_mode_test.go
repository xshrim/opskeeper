package resource

import (
	"context"
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

type accessModeStore struct{ resources map[string]Resource }

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
