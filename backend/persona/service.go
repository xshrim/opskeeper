package persona

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"opskeeper/backend/authorization"
	"opskeeper/backend/engine"
)

type PersonaResolver struct {
	Versions PersonaVersionStore
	Catalog  PersonaCatalog
}

// PersonaService provides the management boundary for versioned personas.
type PersonaService struct {
	Versions PersonaVersionStore
	Catalog  PersonaCatalog
}

type Persona struct {
	ID          string         `json:"id"`
	ScopeID     string         `json:"scope_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Config      map[string]any `json:"config"`
	Status      string         `json:"status"`
}

type PersonaCatalog interface {
	GetPersona(context.Context, string) (Persona, error)
	ListPersonas(context.Context, string) ([]Persona, error)
}

func NewService(versions PersonaVersionStore, catalog PersonaCatalog) *PersonaService {
	return &PersonaService{Versions: versions, Catalog: catalog}
}

func (s *PersonaService) ListPersonas(ctx context.Context, scopeID string) ([]Persona, error) {
	if s.Catalog == nil || !allowsScope(ctx, strings.TrimSpace(scopeID)) {
		return nil, authorization.ErrForbidden
	}
	return s.Catalog.ListPersonas(ctx, strings.TrimSpace(scopeID))
}

func (s *PersonaService) persona(ctx context.Context, id string) (Persona, error) {
	if s == nil || s.Versions == nil {
		return Persona{}, invalid("Persona service is unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Persona{}, invalid("persona_id is required")
	}
	if s.Catalog == nil {
		return Persona{}, invalid("Persona catalog is unavailable")
	}
	item, err := s.Catalog.GetPersona(ctx, id)
	if err != nil {
		return Persona{}, err
	}
	if !allowsScope(ctx, item.ScopeID) {
		return Persona{}, authorization.ErrForbidden
	}
	return item, nil
}

func (s *PersonaService) CreateVersion(ctx context.Context, actorID, personaID string, config map[string]any) (PersonaVersion, error) {
	item, err := s.persona(ctx, personaID)
	if err != nil {
		return PersonaVersion{}, err
	}
	if item.Status != "active" {
		return PersonaVersion{}, invalid("Persona is not active")
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return PersonaVersion{}, invalid("Persona config is invalid")
	}
	var parsed personaConfig
	if err := json.Unmarshal(encoded, &parsed); err != nil {
		return PersonaVersion{}, invalid("Persona config is invalid")
	}
	if err := validatePersonaConfig(parsed); err != nil {
		return PersonaVersion{}, err
	}
	return s.Versions.CreatePersonaVersion(ctx, item.ID, config, strings.TrimSpace(actorID))
}

func (s *PersonaService) ListVersions(ctx context.Context, personaID string) ([]PersonaVersion, error) {
	item, err := s.persona(ctx, personaID)
	if err != nil {
		return nil, err
	}
	return s.Versions.ListPersonaVersions(ctx, item.ID)
}

func (s *PersonaService) PublishVersion(ctx context.Context, personaID, versionID string) (PersonaVersion, error) {
	item, err := s.persona(ctx, personaID)
	if err != nil {
		return PersonaVersion{}, err
	}
	return s.Versions.PublishPersonaVersion(ctx, item.ID, strings.TrimSpace(versionID))
}

func (s *PersonaService) DisableVersion(ctx context.Context, personaID, versionID string) (PersonaVersion, error) {
	item, err := s.persona(ctx, personaID)
	if err != nil {
		return PersonaVersion{}, err
	}
	return s.Versions.DisablePersonaVersion(ctx, item.ID, strings.TrimSpace(versionID))
}

func NewResolver(catalog PersonaCatalog, versions PersonaVersionStore) PersonaResolver {
	return PersonaResolver{Catalog: catalog, Versions: versions}
}

func (r PersonaResolver) Resolve(ctx context.Context, scopeID, personaID string) (engine.Persona, error) {
	scopeID, personaID = strings.TrimSpace(scopeID), strings.TrimSpace(personaID)
	if scopeID == "" || personaID == "" || r.Catalog == nil {
		return engine.Persona{}, invalid("scope_id and persona_id are required")
	}
	var item Persona
	var configSource map[string]any
	var err error
	item, err = r.Catalog.GetPersona(ctx, personaID)
	if err != nil {
		return engine.Persona{}, err
	}
	if !allowsScope(ctx, item.ScopeID) {
		return engine.Persona{}, authorization.ErrForbidden
	}
	configSource = item.Config
	if item.Status != "active" {
		return engine.Persona{}, invalid("Persona is not active")
	}
	if item.ScopeID != scopeID && !allowsScope(ctx, item.ScopeID) {
		return engine.Persona{}, authorization.ErrForbidden
	}
	publishedVersion := 0
	if r.Versions != nil {
		if published, versionErr := r.Versions.GetPublishedPersonaVersion(ctx, item.ID); versionErr == nil {
			configSource = published.Config
			publishedVersion = published.Version
		} else if !errors.Is(versionErr, ErrNotFound) {
			return engine.Persona{}, fmt.Errorf("resolve Persona version: %w", versionErr)
		}
	}
	encoded, err := json.Marshal(configSource)
	if err != nil {
		return engine.Persona{}, fmt.Errorf("encode Persona config: %w", err)
	}
	var config personaConfig
	if err := json.Unmarshal(encoded, &config); err != nil {
		return engine.Persona{}, invalid("Persona config is invalid")
	}
	if err := validatePersonaConfig(config); err != nil {
		return engine.Persona{}, err
	}
	profile := engine.Persona{
		PersonaID: item.ID, ScopeID: item.ScopeID, Name: item.Name,
		Description: config.Description, Version: config.Version,
		Instruction: config.Instruction, Capabilities: config.Capabilities,
		AllowedTools: config.AllowedTools, InputSchema: config.InputSchema,
		OutputSchema: config.OutputSchema, Enabled: config.Enabled,
	}
	if publishedVersion > 0 {
		profile.Version = publishedVersion
	}
	return profile, nil
}

type personaConfig struct {
	Description  string          `json:"description"`
	Version      int             `json:"version"`
	Instruction  string          `json:"instruction"`
	Capabilities []string        `json:"capabilities"`
	AllowedTools []string        `json:"allowed_tools"`
	InputSchema  json.RawMessage `json:"input_schema"`
	OutputSchema json.RawMessage `json:"output_schema"`
	Enabled      bool            `json:"enabled"`
}

var personaToolNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:_-]{0,119}$`)

func validatePersonaConfig(config personaConfig) error {
	if strings.TrimSpace(config.Instruction) == "" {
		return invalid("Persona instruction is required")
	}
	if config.Version < 1 {
		return invalid("Persona version must be positive")
	}
	if len(config.Capabilities) > 30 {
		return invalid("Persona declares at most 30 capabilities")
	}
	seen := make(map[string]struct{}, len(config.Capabilities))
	for _, capability := range config.Capabilities {
		capability = strings.TrimSpace(capability)
		if capability == "" || len(capability) > 80 {
			return invalid("Persona capability is invalid")
		}
		if _, exists := seen[capability]; exists {
			return invalid("Persona capabilities must be unique")
		}
		seen[capability] = struct{}{}
	}
	if len(config.AllowedTools) > 50 {
		return invalid("Persona allows at most 50 tools")
	}
	seen = make(map[string]struct{}, len(config.AllowedTools))
	for _, name := range config.AllowedTools {
		name = strings.TrimSpace(name)
		if !personaToolNamePattern.MatchString(name) {
			return invalid("Persona contains an invalid allowed tool")
		}
		if _, exists := seen[name]; exists {
			return invalid("Persona allowed tools must be unique")
		}
		seen[name] = struct{}{}
	}
	if err := validateObjectSchema(config.InputSchema); err != nil {
		return invalid("Persona input_schema must be an object schema")
	}
	if err := validateObjectSchema(config.OutputSchema); err != nil {
		return invalid("Persona output_schema must be an object schema")
	}
	if !config.Enabled {
		return invalid("Persona is disabled")
	}
	return nil
}

func validatePersona(profile engine.Persona, scopeID string) error {
	if strings.TrimSpace(profile.PersonaID) == "" || strings.TrimSpace(profile.Name) == "" {
		return invalid("Persona persona_id and name are required")
	}
	if scopeID != "" && profile.ScopeID != "" && profile.ScopeID != scopeID {
		return authorization.ErrForbidden
	}
	return validatePersonaConfig(personaConfig{
		Description: profile.Description, Version: profile.Version, Instruction: profile.Instruction,
		Capabilities: profile.Capabilities, AllowedTools: profile.AllowedTools,
		InputSchema: profile.InputSchema, OutputSchema: profile.OutputSchema, Enabled: profile.Enabled,
	})
}

func validateObjectSchema(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return err
	}
	if root, ok := schema["type"]; ok && root != "object" {
		return fmt.Errorf("schema root must be object")
	}
	return nil
}
