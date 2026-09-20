package skill

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"opskeeper/backend/authorization"
	"opskeeper/backend/engine"
	"opskeeper/backend/resource"
)

type ResourceReader interface {
	Get(context.Context, string) (resource.Resource, error)
}

type Service struct {
	store     Store
	resources ResourceReader
	catalog   SkillReader
}

func NewService(store Store, catalog SkillReader, resources ResourceReader) *Service {
	return &Service{store: store, catalog: catalog, resources: resources}
}

func (s *Service) ListSkills(ctx context.Context, scopeID string) ([]Skill, error) {
	if s.catalog == nil || !allowsScope(ctx, strings.TrimSpace(scopeID)) {
		return nil, authorization.ErrForbidden
	}
	return s.catalog.ListSkills(ctx, strings.TrimSpace(scopeID))
}

func (s *Service) CreateVersion(ctx context.Context, actorID string, input CreateVersionInput) (Version, error) {
	input.SkillID = strings.TrimSpace(input.SkillID)
	input.CreatedBy = strings.TrimSpace(actorID)
	input.Manifest.Name = strings.TrimSpace(input.Manifest.Name)
	input.Manifest.Description = strings.TrimSpace(input.Manifest.Description)
	input.Manifest.Instruction = strings.TrimSpace(input.Manifest.Instruction)
	if input.SkillID == "" || input.Manifest.Name == "" || input.Manifest.Instruction == "" {
		return Version{}, invalid("skill_id, manifest.name and manifest.instruction are required")
	}
	item, err := s.skill(ctx, input.SkillID)
	if err != nil {
		return Version{}, err
	}
	if !allowsScope(ctx, item.ScopeID) {
		return Version{}, authorization.ErrForbidden
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
	item, err := s.skill(ctx, strings.TrimSpace(skillID))
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
	item, err := s.skill(ctx, version.SkillID)
	if err != nil {
		return Version{}, err
	}
	if !allowsScope(ctx, item.ScopeID) {
		return Version{}, authorization.ErrForbidden
	}
	return version, nil
}

func (s *Service) Publish(ctx context.Context, skillID, versionID string) (Version, error) {
	item, err := s.skill(ctx, strings.TrimSpace(skillID))
	if err != nil {
		return Version{}, err
	}
	if !allowsScope(ctx, item.ScopeID) {
		return Version{}, authorization.ErrForbidden
	}
	return s.store.PublishVersion(ctx, item.ID, strings.TrimSpace(versionID))
}

func (s *Service) Disable(ctx context.Context, skillID, versionID string) (Version, error) {
	item, err := s.skill(ctx, strings.TrimSpace(skillID))
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
		return Default{}, invalid("scope_id, skill_id and skill_version_id are required")
	}
	if !allowsScope(ctx, scopeID) {
		return Default{}, authorization.ErrForbidden
	}
	version, err := s.store.GetVersion(ctx, versionID)
	if err != nil {
		return Default{}, err
	}
	if version.SkillID != skillID || version.Status != "published" {
		return Default{}, invalid("default Skill version must be published and belong to the Skill")
	}
	return s.store.SetDefault(ctx, Default{ScopeID: scopeID, SkillID: skillID, SkillVersionID: versionID}, strings.TrimSpace(actorID))
}

func (s *Service) Resolve(ctx context.Context, scopeID, explicitSkillID, explicitVersionID string) (Version, error) {
	scopeID = strings.TrimSpace(scopeID)
	if scopeID == "" {
		return Version{}, authorization.ErrForbidden
	}
	versionID := strings.TrimSpace(explicitVersionID)
	if versionID == "" {
		binding, err := s.store.ResolveDefault(ctx, scopeID)
		if err != nil {
			return Version{}, err
		}
		if explicitSkillID != "" && binding.SkillID != strings.TrimSpace(explicitSkillID) {
			return Version{}, ErrNotFound
		}
		versionID = binding.SkillVersionID
	}
	version, err := s.store.GetVersion(ctx, versionID)
	if err != nil {
		return Version{}, err
	}
	if strings.TrimSpace(explicitSkillID) != "" && version.SkillID != strings.TrimSpace(explicitSkillID) {
		return Version{}, ErrNotFound
	}
	if version.Status != "published" {
		return Version{}, invalid("Skill version is not published")
	}
	item, err := s.skill(ctx, version.SkillID)
	if err != nil {
		return Version{}, err
	}
	if !allowsScope(ctx, item.ScopeID) {
		return Version{}, authorization.ErrForbidden
	}
	return version, nil
}

// ResolvePlan exposes a published Skill as a side-effect-free Engine plan.
// It intentionally does not resolve a model, create an execution record or
// invoke a tool; those responsibilities belong exclusively to Engine.
func (s *Service) ResolvePlan(ctx context.Context, scopeID, skillID, versionID string) (engine.ExecutionPlan, error) {
	version, err := s.Resolve(ctx, scopeID, skillID, versionID)
	if err != nil {
		return engine.ExecutionPlan{}, err
	}
	declarations := make([]engine.ToolDeclaration, 0, len(version.Tools))
	allowed := make([]string, 0, len(version.Tools))
	for _, item := range version.Tools {
		declarations = append(declarations, engine.ToolDeclaration{Name: item.Name, Description: item.Description, InputSchema: item.InputSchema})
		allowed = append(allowed, item.Name)
	}
	return engine.ExecutionPlan{
		SourceSkillID:   version.SkillID,
		SourceVersionID: version.ID,
		Name:            version.Manifest.Name,
		Description:     version.Manifest.Description,
		Instruction:     version.Manifest.Instruction,
		InputSchema:     version.InputSchema,
		OutputSchema:    version.OutputSchema,
		Tools:           declarations,
		AllowedTools:    allowed,
	}, nil
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
	if filter, ok := authorization.ResourceFilterFromContext(ctx); ok && !filter.Allows(item.ScopeID, item.ID) {
		return resource.Resource{}, authorization.ErrForbidden
	}
	return item, nil
}

func (s *Service) skill(ctx context.Context, id string) (Skill, error) {
	if s.catalog == nil || strings.TrimSpace(id) == "" {
		return Skill{}, invalid("skill_id is required")
	}
	item, err := s.catalog.GetSkill(ctx, strings.TrimSpace(id))
	if err != nil {
		return Skill{}, err
	}
	if !allowsScope(ctx, item.ScopeID) {
		return Skill{}, authorization.ErrForbidden
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
	case "connector_kubernetes_read", "connector_metrics_query", "connector_logs_query", "connector_traces_query", "connector_alerts_get", "connector_postgresql_inspect", "connector_kafka_inspect", "kafka_health", "kafka_brokers", "kafka_topics", "kafka_consumer_groups", "kafka_consumer_lag", "kafka_cluster_info", "kafka_topic_partitions", "redis_health", "redis_memory", "redis_clients", "redis_replication", "redis_slowlog", "redis_database_info", "postgresql_health", "postgresql_sessions", "postgresql_long_running_queries", "postgresql_locks", "postgresql_replication", "postgresql_capacity", "postgresql_tables", "postgresql_table_columns", "postgresql_performance", "postgresql_vacuum", "postgresql_extensions", "postgresql_database_info", "mysql_health", "mysql_status", "mysql_performance", "mysql_tables", "mysql_table_columns", "mysql_table_structure", "mysql_database_info":
		return true
	default:
		return false
	}
}

func allowsScope(ctx context.Context, scopeID string) bool {
	filter, ok := authorization.ScopeFilterFromContext(ctx)
	return !ok || filter.Allows(scopeID)
}
