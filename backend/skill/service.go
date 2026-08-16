package skill

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"opskeeper/backend/authorization"
	"opskeeper/backend/resource"
)

type ResourceReader interface {
	Get(context.Context, string) (resource.Resource, error)
}

type Service struct {
	store     Store
	resources ResourceReader
}

func NewService(store Store, resources ResourceReader) *Service {
	return &Service{store: store, resources: resources}
}

func (s *Service) CreateVersion(ctx context.Context, actorID string, input CreateVersionInput) (Version, error) {
	input.SkillResourceID = strings.TrimSpace(input.SkillResourceID)
	input.CreatedBy = strings.TrimSpace(actorID)
	input.Manifest.Name = strings.TrimSpace(input.Manifest.Name)
	input.Manifest.Description = strings.TrimSpace(input.Manifest.Description)
	input.Manifest.Instruction = strings.TrimSpace(input.Manifest.Instruction)
	if input.SkillResourceID == "" || input.Manifest.Name == "" || input.Manifest.Instruction == "" {
		return Version{}, invalid("skill_resource_id, manifest.name and manifest.instruction are required")
	}
	item, err := s.skillResource(ctx, input.SkillResourceID)
	if err != nil {
		return Version{}, err
	}
	if !allowsScope(ctx, item.ScopeID) {
		return Version{}, authorization.ErrForbidden
	}
	if len(input.Manifest.TargetKinds) == 0 || len(input.Manifest.TargetKinds) > 50 {
		return Version{}, invalid("manifest.target_kinds must contain 1 to 50 resource kinds")
	}
	if len(input.Tools) > 20 {
		return Version{}, invalid("a Skill version may declare at most 20 tools")
	}
	names := make([]string, 0, len(input.Tools))
	for index := range input.Tools {
		input.Tools[index].Name = strings.TrimSpace(input.Tools[index].Name)
		if input.Tools[index].Name == "" || !allowedToolName(input.Tools[index].Name) {
			return Version{}, invalid("Skill contains an unsupported tool")
		}
		if slices.Contains(names, input.Tools[index].Name) {
			return Version{}, invalid("Skill tool names must be unique")
		}
		names = append(names, input.Tools[index].Name)
		if err := validateSchema(input.Tools[index].InputSchema); err != nil {
			return Version{}, invalid("Skill tool input_schema must be a JSON object schema")
		}
	}
	if err := validateSchema(input.InputSchema); err != nil {
		return Version{}, invalid("input_schema must be a JSON object schema")
	}
	if err := validateSchema(input.OutputSchema); err != nil {
		return Version{}, invalid("output_schema must be a JSON object schema")
	}
	if input.RiskLevel != "read_only" && input.RiskLevel != "controlled" && input.RiskLevel != "high" {
		return Version{}, invalid("risk_level must be read_only, controlled or high")
	}
	return s.store.CreateVersion(ctx, input)
}

func (s *Service) ListVersions(ctx context.Context, skillID string) ([]Version, error) {
	item, err := s.skillResource(ctx, strings.TrimSpace(skillID))
	if err != nil {
		return nil, err
	}
	if !allowsScope(ctx, item.ScopeID) {
		return nil, authorization.ErrForbidden
	}
	return s.store.ListVersions(ctx, item.ID)
}

func (s *Service) GetVersion(ctx context.Context, versionID string) (Version, error) {
	version, err := s.store.GetVersion(ctx, strings.TrimSpace(versionID))
	if err != nil {
		return Version{}, err
	}
	item, err := s.skillResource(ctx, version.SkillResourceID)
	if err != nil {
		return Version{}, err
	}
	if !allowsScope(ctx, item.ScopeID) {
		return Version{}, authorization.ErrForbidden
	}
	return version, nil
}

func (s *Service) Publish(ctx context.Context, skillID, versionID string) (Version, error) {
	item, err := s.skillResource(ctx, strings.TrimSpace(skillID))
	if err != nil {
		return Version{}, err
	}
	if !allowsScope(ctx, item.ScopeID) {
		return Version{}, authorization.ErrForbidden
	}
	return s.store.PublishVersion(ctx, item.ID, strings.TrimSpace(versionID))
}

func (s *Service) Disable(ctx context.Context, skillID, versionID string) (Version, error) {
	item, err := s.skillResource(ctx, strings.TrimSpace(skillID))
	if err != nil {
		return Version{}, err
	}
	if !allowsScope(ctx, item.ScopeID) {
		return Version{}, authorization.ErrForbidden
	}
	return s.store.DisableVersion(ctx, item.ID, strings.TrimSpace(versionID))
}

func (s *Service) SetDefault(ctx context.Context, actorID, scopeID, skillID, versionID string) (Default, error) {
	scopeID, skillID, versionID = strings.TrimSpace(scopeID), strings.TrimSpace(skillID), strings.TrimSpace(versionID)
	if scopeID == "" || skillID == "" || versionID == "" {
		return Default{}, invalid("scope_id, skill_resource_id and skill_version_id are required")
	}
	if !allowsScope(ctx, scopeID) {
		return Default{}, authorization.ErrForbidden
	}
	version, err := s.store.GetVersion(ctx, versionID)
	if err != nil {
		return Default{}, err
	}
	if version.SkillResourceID != skillID || version.Status != "published" {
		return Default{}, invalid("default Skill version must be published and belong to the Skill")
	}
	return s.store.SetDefault(ctx, Default{ScopeID: scopeID, SkillResourceID: skillID, SkillVersionID: versionID}, strings.TrimSpace(actorID))
}

func (s *Service) Resolve(ctx context.Context, scopeID, explicitSkillID, explicitVersionID string) (Version, error) {
	scopeID = strings.TrimSpace(scopeID)
	if scopeID == "" || !allowsScope(ctx, scopeID) {
		return Version{}, authorization.ErrForbidden
	}
	versionID := strings.TrimSpace(explicitVersionID)
	if versionID == "" {
		binding, err := s.store.ResolveDefault(ctx, scopeID)
		if err != nil {
			return Version{}, err
		}
		if explicitSkillID != "" && binding.SkillResourceID != strings.TrimSpace(explicitSkillID) {
			return Version{}, ErrNotFound
		}
		versionID = binding.SkillVersionID
	}
	version, err := s.store.GetVersion(ctx, versionID)
	if err != nil {
		return Version{}, err
	}
	if strings.TrimSpace(explicitSkillID) != "" && version.SkillResourceID != strings.TrimSpace(explicitSkillID) {
		return Version{}, ErrNotFound
	}
	if version.Status != "published" {
		return Version{}, invalid("Skill version is not published")
	}
	return version, nil
}

func (s *Service) ValidateTarget(ctx context.Context, version Version, targetID string) (resource.Resource, error) {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return resource.Resource{}, invalid("target_resource_id is required")
	}
	item, err := s.resources.Get(ctx, targetID)
	if err != nil {
		return resource.Resource{}, err
	}
	if !slices.Contains(version.Manifest.TargetKinds, item.Kind) {
		return resource.Resource{}, invalid("Skill does not support the target resource kind")
	}
	if filter, ok := authorization.ResourceFilterFromContext(ctx); ok && !filter.Allows(item.ScopeID, item.ID) {
		return resource.Resource{}, authorization.ErrForbidden
	}
	return item, nil
}

func (s *Service) GetExecution(ctx context.Context, id string) (Execution, error) {
	item, err := s.store.GetExecution(ctx, strings.TrimSpace(id))
	if err != nil {
		return Execution{}, err
	}
	if !allowsScope(ctx, item.ScopeID) {
		return Execution{}, authorization.ErrForbidden
	}
	return item, nil
}

func (s *Service) ListExecutions(ctx context.Context, scopeID string, limit int) ([]Execution, error) {
	if !allowsScope(ctx, strings.TrimSpace(scopeID)) {
		return nil, authorization.ErrForbidden
	}
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 200 {
		return nil, invalid("limit must be between 1 and 200")
	}
	return s.store.ListExecutions(ctx, scopeID, limit)
}

func (s *Service) skillResource(ctx context.Context, id string) (resource.Resource, error) {
	if id == "" || s.resources == nil {
		return resource.Resource{}, invalid("skill_resource_id is required")
	}
	item, err := s.resources.Get(ctx, id)
	if err != nil {
		return resource.Resource{}, err
	}
	if item.Kind != Kind {
		return resource.Resource{}, invalid("resource is not a Skill")
	}
	return item, nil
}

func validateSchema(raw json.RawMessage) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{"type":"object"}`)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return err
	}
	if schema["type"] != nil && schema["type"] != "object" {
		return invalid("schema root must be object")
	}
	return nil
}

func allowedToolName(name string) bool {
	switch name {
	case "connector_kubernetes_read", "connector_metrics_query", "connector_logs_query", "connector_traces_query", "connector_alerts_get", "connector_postgresql_inspect", "connector_redis_inspect", "connector_kafka_inspect":
		return true
	default:
		return false
	}
}

func allowsScope(ctx context.Context, scopeID string) bool {
	filter, ok := authorization.ScopeFilterFromContext(ctx)
	return !ok || filter.Allows(scopeID)
}
