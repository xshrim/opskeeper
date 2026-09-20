package persona

import (
	"context"
	"errors"
	"strings"
	"testing"

	"opskeeper/backend/authorization"
	"opskeeper/backend/engine"
)

type fakePersonaCatalog struct{ items map[string]Persona }

func (f fakePersonaCatalog) GetPersona(_ context.Context, id string) (Persona, error) {
	item, ok := f.items[id]
	if !ok {
		return Persona{}, ErrNotFound
	}
	return item, nil
}
func (f fakePersonaCatalog) ListPersonas(context.Context, string) ([]Persona, error) { return nil, nil }

func TestPersonaResolverValidatesAndReturnsContract(t *testing.T) {
	catalog := fakePersonaCatalog{items: map[string]Persona{
		"profile-1": {ID: "profile-1", ScopeID: "scope-1", Status: "active", Name: "PostgreSQL expert", Config: map[string]any{
			"version": 2, "instruction": "Inspect database evidence and explain the findings.",
			"description": "Database specialist", "capabilities": []any{"text", "tool_calling"},
			"allowed_tools": []any{"connector_postgresql_inspect"}, "enabled": true,
			"input_schema": map[string]any{"type": "object"},
		}},
	}}
	profile, err := NewResolver(catalog, nil).Resolve(context.Background(), "scope-1", "profile-1")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if profile.Name != "PostgreSQL expert" || profile.Version != 2 || len(profile.AllowedTools) != 1 {
		t.Fatalf("profile = %+v", profile)
	}
}

func TestPersonaResolverRejectsUnauthorizedAndDisabledProfiles(t *testing.T) {
	catalog := fakePersonaCatalog{items: map[string]Persona{
		"profile-1": {ID: "profile-1", ScopeID: "scope-1", Status: "active", Config: map[string]any{"version": 1, "instruction": "x", "enabled": true}},
		"profile-2": {ID: "profile-2", ScopeID: "scope-1", Status: "disabled", Config: map[string]any{"version": 1, "instruction": "x", "enabled": true}},
	}}
	resolver := NewResolver(catalog, nil)
	denied := authorization.WithScopeFilter(context.Background(), authorization.ScopeFilter{ScopeIDs: []string{"other"}})
	if _, err := resolver.Resolve(denied, "scope-1", "profile-1"); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("unauthorized Resolve() error = %v", err)
	}
	if _, err := resolver.Resolve(context.Background(), "scope-1", "profile-2"); err == nil {
		t.Fatal("disabled profile was accepted")
	}
}

func TestPersonaResolverRejectsInvalidContract(t *testing.T) {
	catalog := fakePersonaCatalog{items: map[string]Persona{
		"profile-1": {ID: "profile-1", ScopeID: "scope-1", Status: "active", Config: map[string]any{"version": 1, "instruction": "x", "allowed_tools": []any{"bad tool"}, "enabled": true}},
	}}
	_, err := NewResolver(catalog, nil).Resolve(context.Background(), "scope-1", "profile-1")
	if err == nil || !strings.Contains(err.Error(), "invalid allowed tool") {
		t.Fatalf("invalid contract error = %v", err)
	}
}

func TestPersonaResolverUsesPublishedVersion(t *testing.T) {
	catalog := fakePersonaCatalog{items: map[string]Persona{
		"profile-1": {ID: "profile-1", ScopeID: "scope-1", Status: "active", Name: "expert", Config: map[string]any{"version": 1, "instruction": "old", "enabled": true}},
	}}
	versions := fakePersonaVersionStore{published: PersonaVersion{ID: "version-2", PersonaID: "profile-1", Version: 2, Config: map[string]any{"version": 1, "instruction": "new", "enabled": true}}}
	profile, err := NewResolver(catalog, versions).Resolve(context.Background(), "scope-1", "profile-1")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if profile.Version != 2 || profile.Instruction != "new" {
		t.Fatalf("published profile = %+v", profile)
	}
}

func TestValidatePersonaChecksDirectRunnerProfiles(t *testing.T) {
	profile := engine.Persona{PersonaID: "profile-1", ScopeID: "scope-1", Name: "expert", Version: 1, Instruction: "x", Enabled: false}
	if err := validatePersona(profile, "scope-1"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("expected disabled profile error, got %v", err)
	}
	profile.Enabled = true
	profile.ScopeID = "scope-2"
	if err := validatePersona(profile, "scope-1"); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("expected scope mismatch forbidden, got %v", err)
	}
}

type fakePersonaVersionStore struct{ published PersonaVersion }

func (f fakePersonaVersionStore) CreatePersonaVersion(context.Context, string, map[string]any, string) (PersonaVersion, error) {
	return PersonaVersion{}, nil
}
func (f fakePersonaVersionStore) ListPersonaVersions(context.Context, string) ([]PersonaVersion, error) {
	return nil, nil
}
func (f fakePersonaVersionStore) GetPublishedPersonaVersion(context.Context, string) (PersonaVersion, error) {
	return f.published, nil
}
func (f fakePersonaVersionStore) PublishPersonaVersion(context.Context, string, string) (PersonaVersion, error) {
	return f.published, nil
}
func (f fakePersonaVersionStore) DisablePersonaVersion(context.Context, string, string) (PersonaVersion, error) {
	return f.published, nil
}
