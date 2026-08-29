package skill

import (
	"context"
	"errors"
	"strings"
	"testing"

	"opskeeper/backend/aiengine"
	"opskeeper/backend/authorization"
	"opskeeper/backend/resource"
)

func TestAgentProfileResolverValidatesAndReturnsContract(t *testing.T) {
	resources := fakeAgentProfileResources{items: map[string]resource.Resource{
		"profile-1": {
			ID: "profile-1", ScopeID: "scope-1", Kind: AgentProfileKind, Status: resource.StatusActive, Name: "PostgreSQL expert",
			Config: map[string]any{
				"version": 2, "instruction": "Inspect database evidence and explain the findings.",
				"description": "Database specialist", "capabilities": []any{"text", "tool_calling"},
				"allowed_tools": []any{"connector_postgresql_inspect"}, "target_kinds": []any{"PostgreSQL"}, "enabled": true,
				"input_schema": map[string]any{"type": "object"},
			},
		},
	}}
	resolver := NewAgentProfileResolver(resources)
	profile, err := resolver.Resolve(authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{ResourceIDs: []string{"profile-1"}}), "scope-1", "profile-1")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if profile.Name != "PostgreSQL expert" || profile.Version != 2 || len(profile.AllowedTools) != 1 || profile.TargetKinds[0] != "PostgreSQL" {
		t.Fatalf("profile = %+v", profile)
	}
}

func TestAgentProfileResolverRejectsUnauthorizedAndDisabledProfiles(t *testing.T) {
	resources := fakeAgentProfileResources{items: map[string]resource.Resource{
		"profile-1": {ID: "profile-1", ScopeID: "scope-1", Kind: AgentProfileKind, Status: resource.StatusActive, Config: map[string]any{"version": 1, "instruction": "x", "enabled": true}},
		"profile-2": {ID: "profile-2", ScopeID: "scope-1", Kind: AgentProfileKind, Status: resource.StatusDisabled, Config: map[string]any{"version": 1, "instruction": "x", "enabled": true}},
	}}
	resolver := NewAgentProfileResolver(resources)
	denied := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{ResourceIDs: []string{"other"}})
	if _, err := resolver.Resolve(denied, "scope-1", "profile-1"); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("unauthorized Resolve() error = %v", err)
	}
	allowed := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{ResourceIDs: []string{"profile-2"}})
	if _, err := resolver.Resolve(allowed, "scope-1", "profile-2"); err == nil {
		t.Fatal("disabled profile was accepted")
	}
}

func TestAgentProfileResolverRejectsInvalidContract(t *testing.T) {
	resources := fakeAgentProfileResources{items: map[string]resource.Resource{
		"profile-1": {ID: "profile-1", ScopeID: "scope-1", Kind: AgentProfileKind, Status: resource.StatusActive, Config: map[string]any{"version": 1, "instruction": "x", "allowed_tools": []any{"bad tool"}, "enabled": true}},
	}}
	_, err := NewAgentProfileResolver(resources).Resolve(context.Background(), "scope-1", "profile-1")
	if err == nil || !strings.Contains(err.Error(), "invalid allowed tool") {
		t.Fatalf("invalid contract error = %v", err)
	}
}

func TestAgentProfileResolverUsesPublishedVersion(t *testing.T) {
	resources := fakeAgentProfileResources{items: map[string]resource.Resource{
		"profile-1": {ID: "profile-1", ScopeID: "scope-1", Kind: AgentProfileKind, Status: resource.StatusActive, Name: "expert", Config: map[string]any{"version": 1, "instruction": "old", "enabled": true}},
	}}
	versions := fakeAgentProfileVersionStore{published: AgentProfileVersion{ID: "version-2", ProfileResourceID: "profile-1", Version: 2, Config: map[string]any{"version": 1, "instruction": "new", "enabled": true}}}
	profile, err := (AgentProfileResolver{Resources: resources, Versions: versions}).Resolve(context.Background(), "scope-1", "profile-1")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if profile.Version != 2 || profile.Instruction != "new" {
		t.Fatalf("published profile = %+v", profile)
	}
}

func TestValidateAgentProfileChecksDirectRunnerProfiles(t *testing.T) {
	profile := aiengine.AgentProfile{ResourceID: "profile-1", ScopeID: "scope-1", Name: "expert", Version: 1, Instruction: "x", Enabled: false}
	if err := validateAgentProfile(profile, "scope-1"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("expected disabled profile error, got %v", err)
	}
	profile.Enabled = true
	profile.ScopeID = "scope-2"
	if err := validateAgentProfile(profile, "scope-1"); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("expected scope mismatch forbidden, got %v", err)
	}
}

type fakeAgentProfileResources struct{ items map[string]resource.Resource }

type fakeAgentProfileVersionStore struct{ published AgentProfileVersion }

func (f fakeAgentProfileVersionStore) CreateAgentProfileVersion(context.Context, string, map[string]any, string) (AgentProfileVersion, error) {
	return AgentProfileVersion{}, nil
}
func (f fakeAgentProfileVersionStore) ListAgentProfileVersions(context.Context, string) ([]AgentProfileVersion, error) {
	return nil, nil
}
func (f fakeAgentProfileVersionStore) GetPublishedAgentProfileVersion(context.Context, string) (AgentProfileVersion, error) {
	return f.published, nil
}
func (f fakeAgentProfileVersionStore) PublishAgentProfileVersion(context.Context, string, string) (AgentProfileVersion, error) {
	return f.published, nil
}
func (f fakeAgentProfileVersionStore) DisableAgentProfileVersion(context.Context, string, string) (AgentProfileVersion, error) {
	return f.published, nil
}

func (f fakeAgentProfileResources) Get(_ context.Context, id string) (resource.Resource, error) {
	item, ok := f.items[id]
	if !ok {
		return resource.Resource{}, resource.ErrNotFound
	}
	return item, nil
}
