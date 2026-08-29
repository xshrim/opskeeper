package skill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"opskeeper/backend/aiengine"
	"opskeeper/backend/authorization"
	"opskeeper/backend/resource"
)

// AgentProfileKind is a resource-backed expert agent definition. Profiles are
// deliberately kept in the resource catalog so existing scope and resource
// permissions apply without introducing a second authorization model.
const AgentProfileKind = "AgentProfile"

type AgentProfileResolver struct {
	Resources ResourceReader
	Versions  AgentProfileVersionStore
}

// AgentProfileService provides the management boundary for versioned profiles.
// Resource metadata remains managed by the generic Resource API; this service
// owns only immutable contract snapshots and their lifecycle.
type AgentProfileService struct {
	Resources ResourceReader
	Versions  AgentProfileVersionStore
}

func NewAgentProfileService(resources ResourceReader, versions AgentProfileVersionStore) *AgentProfileService {
	return &AgentProfileService{Resources: resources, Versions: versions}
}

func (s *AgentProfileService) profile(ctx context.Context, id string) (resource.Resource, error) {
	if s == nil || s.Resources == nil || s.Versions == nil {
		return resource.Resource{}, invalid("AgentProfile service is unavailable")
	}
	item, err := s.Resources.Get(ctx, strings.TrimSpace(id))
	if err != nil {
		return resource.Resource{}, err
	}
	if item.Kind != AgentProfileKind {
		return resource.Resource{}, invalid("resource is not an AgentProfile")
	}
	if !allowsProfile(ctx, item) {
		return resource.Resource{}, authorization.ErrForbidden
	}
	return item, nil
}

func (s *AgentProfileService) CreateVersion(ctx context.Context, actorID, profileID string, config map[string]any) (AgentProfileVersion, error) {
	item, err := s.profile(ctx, profileID)
	if err != nil {
		return AgentProfileVersion{}, err
	}
	if item.Status != resource.StatusActive {
		return AgentProfileVersion{}, invalid("AgentProfile is not active")
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return AgentProfileVersion{}, invalid("AgentProfile config is invalid")
	}
	var parsed agentProfileConfig
	if err := json.Unmarshal(encoded, &parsed); err != nil {
		return AgentProfileVersion{}, invalid("AgentProfile config is invalid")
	}
	if err := validateAgentProfileConfig(parsed); err != nil {
		return AgentProfileVersion{}, err
	}
	return s.Versions.CreateAgentProfileVersion(ctx, item.ID, config, strings.TrimSpace(actorID))
}

func (s *AgentProfileService) ListVersions(ctx context.Context, profileID string) ([]AgentProfileVersion, error) {
	item, err := s.profile(ctx, profileID)
	if err != nil {
		return nil, err
	}
	return s.Versions.ListAgentProfileVersions(ctx, item.ID)
}

func (s *AgentProfileService) PublishVersion(ctx context.Context, profileID, versionID string) (AgentProfileVersion, error) {
	item, err := s.profile(ctx, profileID)
	if err != nil {
		return AgentProfileVersion{}, err
	}
	return s.Versions.PublishAgentProfileVersion(ctx, item.ID, strings.TrimSpace(versionID))
}

func (s *AgentProfileService) DisableVersion(ctx context.Context, profileID, versionID string) (AgentProfileVersion, error) {
	item, err := s.profile(ctx, profileID)
	if err != nil {
		return AgentProfileVersion{}, err
	}
	return s.Versions.DisableAgentProfileVersion(ctx, item.ID, strings.TrimSpace(versionID))
}

func NewAgentProfileResolver(resources ResourceReader) AgentProfileResolver {
	return AgentProfileResolver{Resources: resources}
}

func (r AgentProfileResolver) Resolve(ctx context.Context, scopeID, profileID string) (aiengine.AgentProfile, error) {
	scopeID, profileID = strings.TrimSpace(scopeID), strings.TrimSpace(profileID)
	if scopeID == "" || profileID == "" || r.Resources == nil {
		return aiengine.AgentProfile{}, invalid("scope_id and agent_profile_id are required")
	}
	item, err := r.Resources.Get(ctx, profileID)
	if err != nil {
		return aiengine.AgentProfile{}, err
	}
	if item.Kind != AgentProfileKind {
		return aiengine.AgentProfile{}, invalid("resource is not an AgentProfile")
	}
	if item.Status != resource.StatusActive {
		return aiengine.AgentProfile{}, invalid("AgentProfile is not active")
	}
	if !allowsProfile(ctx, item) || item.ScopeID != scopeID && !allowsScope(ctx, item.ScopeID) {
		return aiengine.AgentProfile{}, authorization.ErrForbidden
	}
	configSource := item.Config
	publishedVersion := 0
	if r.Versions != nil {
		if published, versionErr := r.Versions.GetPublishedAgentProfileVersion(ctx, item.ID); versionErr == nil {
			configSource = published.Config
			publishedVersion = published.Version
		} else if !errors.Is(versionErr, ErrNotFound) {
			return aiengine.AgentProfile{}, fmt.Errorf("resolve AgentProfile version: %w", versionErr)
		}
	}
	encoded, err := json.Marshal(configSource)
	if err != nil {
		return aiengine.AgentProfile{}, fmt.Errorf("encode AgentProfile config: %w", err)
	}
	var config agentProfileConfig
	if err := json.Unmarshal(encoded, &config); err != nil {
		return aiengine.AgentProfile{}, invalid("AgentProfile config is invalid")
	}
	if err := validateAgentProfileConfig(config); err != nil {
		return aiengine.AgentProfile{}, err
	}
	profile := aiengine.AgentProfile{
		ResourceID: item.ID, ScopeID: item.ScopeID, Name: item.Name,
		Description: config.Description, Version: config.Version,
		Instruction: config.Instruction, Capabilities: config.Capabilities,
		AllowedTools: config.AllowedTools, TargetKinds: config.TargetKinds, InputSchema: config.InputSchema,
		OutputSchema: config.OutputSchema, Enabled: config.Enabled,
	}
	if publishedVersion > 0 {
		profile.Version = publishedVersion
	}
	return profile, nil
}

type agentProfileConfig struct {
	Description  string          `json:"description"`
	Version      int             `json:"version"`
	Instruction  string          `json:"instruction"`
	Capabilities []string        `json:"capabilities"`
	AllowedTools []string        `json:"allowed_tools"`
	TargetKinds  []string        `json:"target_kinds"`
	InputSchema  json.RawMessage `json:"input_schema"`
	OutputSchema json.RawMessage `json:"output_schema"`
	Enabled      bool            `json:"enabled"`
}

var agentToolNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:_-]{0,119}$`)

func validateAgentProfileConfig(config agentProfileConfig) error {
	if strings.TrimSpace(config.Instruction) == "" {
		return invalid("AgentProfile instruction is required")
	}
	if config.Version < 1 {
		return invalid("AgentProfile version must be positive")
	}
	if len(config.Capabilities) > 30 {
		return invalid("AgentProfile declares at most 30 capabilities")
	}
	seen := make(map[string]struct{}, len(config.Capabilities))
	for _, capability := range config.Capabilities {
		capability = strings.TrimSpace(capability)
		if capability == "" || len(capability) > 80 {
			return invalid("AgentProfile capability is invalid")
		}
		if _, exists := seen[capability]; exists {
			return invalid("AgentProfile capabilities must be unique")
		}
		seen[capability] = struct{}{}
	}
	if len(config.AllowedTools) > 50 {
		return invalid("AgentProfile allows at most 50 tools")
	}
	seen = make(map[string]struct{}, len(config.AllowedTools))
	for _, name := range config.AllowedTools {
		name = strings.TrimSpace(name)
		if !agentToolNamePattern.MatchString(name) {
			return invalid("AgentProfile contains an invalid allowed tool")
		}
		if _, exists := seen[name]; exists {
			return invalid("AgentProfile allowed tools must be unique")
		}
		seen[name] = struct{}{}
	}
	if err := validateObjectSchema(config.InputSchema); err != nil {
		return invalid("AgentProfile input_schema must be an object schema")
	}
	if err := validateObjectSchema(config.OutputSchema); err != nil {
		return invalid("AgentProfile output_schema must be an object schema")
	}
	if !config.Enabled {
		return invalid("AgentProfile is disabled")
	}
	return nil
}

func validateAgentProfile(profile aiengine.AgentProfile, scopeID string) error {
	if strings.TrimSpace(profile.ResourceID) == "" || strings.TrimSpace(profile.Name) == "" {
		return invalid("AgentProfile resource_id and name are required")
	}
	if scopeID != "" && profile.ScopeID != "" && profile.ScopeID != scopeID {
		return authorization.ErrForbidden
	}
	return validateAgentProfileConfig(agentProfileConfig{
		Description: profile.Description, Version: profile.Version, Instruction: profile.Instruction,
		Capabilities: profile.Capabilities, AllowedTools: profile.AllowedTools, TargetKinds: profile.TargetKinds,
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

func allowsProfile(ctx context.Context, item resource.Resource) bool {
	if filter, ok := authorization.ResourceFilterFromContext(ctx); ok {
		return filter.Allows(item.ScopeID, item.ID)
	}
	return allowsScope(ctx, item.ScopeID)
}
