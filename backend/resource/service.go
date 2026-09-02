package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"opskeeper/backend/authorization"
)

type Store interface {
	Create(context.Context, CreateInput) (Resource, error)
	UpsertImported(context.Context, ImportedInput) (Resource, error)
	List(context.Context, Pagination, string, map[string]string) (Page[Resource], error)
	Get(context.Context, string) (Resource, error)
	Update(context.Context, string, UpdateInput) (Resource, error)
	Delete(context.Context, string) error
	GetSchema(context.Context, string, int) (Schema, error)
	ListSchemas(context.Context) ([]Schema, error)
	CreateRelation(context.Context, CreateRelationInput, string) (Relation, error)
	ListRelations(context.Context, string) ([]Relation, error)
	DeleteRelation(context.Context, string, string) error
	Topology(context.Context, string, int, int) ([]TopologyNode, error)
	SetDefault(context.Context, string, string, string) (Default, error)
	ResolveDefault(context.Context, string, string) (Resource, error)
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) Create(ctx context.Context, input CreateInput) (Resource, error) {
	input.ScopeID = strings.TrimSpace(input.ScopeID)
	input.Kind = strings.TrimSpace(input.Kind)
	input.Name = strings.TrimSpace(input.Name)
	input.ExternalUID = strings.TrimSpace(input.ExternalUID)
	input.SourceResourceID = strings.TrimSpace(input.SourceResourceID)
	if err := validateResourceInput(input.ScopeID, input.Kind, input.Name); err != nil {
		return Resource{}, err
	}
	input.Subtype = normalizeResourceSubtype(input.Kind, input.Subtype)
	if err := validateResourceSubtype(input.Kind, input.Subtype); err != nil {
		return Resource{}, err
	}
	if !allowsExactScope(ctx, input.ScopeID) {
		return Resource{}, authorization.ErrForbidden
	}
	if input.Status == "" {
		input.Status = StatusActive
	}
	if err := validateStatus(input.Status); err != nil {
		return Resource{}, err
	}
	if err := validateLabels(input.Labels); err != nil {
		return Resource{}, err
	}
	if input.Labels == nil {
		input.Labels = map[string]string{}
	}
	if input.Config == nil {
		input.Config = map[string]any{}
	}
	schema, err := s.store.GetSchema(ctx, input.Kind, input.SchemaVersion)
	if err != nil {
		return Resource{}, err
	}
	if err := validateConfig(input.Config, schema); err != nil {
		return Resource{}, err
	}
	if input.Kind == "AIProvider" {
		if err := validateAIProviderConfig(input.Config); err != nil {
			return Resource{}, err
		}
	}
	if input.Kind == "Workflow" {
		if err := validateWorkflowConfig(input.Config); err != nil {
			return Resource{}, err
		}
	}
	input.SchemaVersion = schema.Version
	return s.store.Create(ctx, input)
}

// Import persists a resource discovered from an external system. The source
// identity is part of the input so repeated discovery runs update one record.
func (s *Service) Import(ctx context.Context, input ImportedInput) (Resource, error) {
	input.ScopeID = strings.TrimSpace(input.ScopeID)
	input.Kind = strings.TrimSpace(input.Kind)
	input.Name = strings.TrimSpace(input.Name)
	input.ExternalUID = strings.TrimSpace(input.ExternalUID)
	input.SourceResourceID = strings.TrimSpace(input.SourceResourceID)
	if err := validateResourceInput(input.ScopeID, input.Kind, input.Name); err != nil {
		return Resource{}, err
	}
	input.Subtype = normalizeResourceSubtype(input.Kind, input.Subtype)
	if err := validateResourceSubtype(input.Kind, input.Subtype); err != nil {
		return Resource{}, err
	}
	if input.ExternalUID == "" || input.SourceResourceID == "" {
		return Resource{}, invalid("imported resources require external_uid and source_resource_id")
	}
	if !allowsExactScope(ctx, input.ScopeID) {
		return Resource{}, authorization.ErrForbidden
	}
	if input.Status == "" {
		input.Status = StatusActive
	}
	if err := validateStatus(input.Status); err != nil {
		return Resource{}, err
	}
	if err := validateLabels(input.Labels); err != nil {
		return Resource{}, err
	}
	if input.Labels == nil {
		input.Labels = map[string]string{}
	}
	if input.Config == nil {
		input.Config = map[string]any{}
	}
	schema, err := s.store.GetSchema(ctx, input.Kind, input.SchemaVersion)
	if err != nil {
		return Resource{}, err
	}
	if err := validateConfig(input.Config, schema); err != nil {
		return Resource{}, err
	}
	if input.Kind == "AIProvider" {
		if err := validateAIProviderConfig(input.Config); err != nil {
			return Resource{}, err
		}
	}
	input.SchemaVersion = schema.Version
	return s.store.UpsertImported(ctx, input)
}

func (s *Service) List(ctx context.Context, pagination Pagination, kind string, labels map[string]string) (Page[Resource], error) {
	pagination, err := normalizePagination(pagination)
	if err != nil {
		return Page[Resource]{}, err
	}
	return s.store.List(ctx, pagination, strings.TrimSpace(kind), labels)
}

func (s *Service) Get(ctx context.Context, id string) (Resource, error) {
	if strings.TrimSpace(id) == "" {
		return Resource{}, invalid("resource_id is required")
	}
	return s.store.Get(ctx, id)
}

func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (Resource, error) {
	if strings.TrimSpace(id) == "" {
		return Resource{}, invalid("resource_id is required")
	}
	if input.ScopeID != nil {
		value := strings.TrimSpace(*input.ScopeID)
		if value == "" || !allowsExactScope(ctx, value) {
			return Resource{}, authorization.ErrForbidden
		}
		input.ScopeID = &value
	}
	if input.Name != nil {
		value := strings.TrimSpace(*input.Name)
		if value == "" || len([]rune(value)) > 200 {
			return Resource{}, invalid("name must contain 1 to 200 characters")
		}
		input.Name = &value
	}
	if input.Status != nil {
		value := strings.TrimSpace(*input.Status)
		if err := validateStatus(value); err != nil {
			return Resource{}, err
		}
		input.Status = &value
	}
	if input.Subtype != nil {
		value := strings.TrimSpace(*input.Subtype)
		current, err := s.store.Get(ctx, id)
		if err != nil {
			return Resource{}, err
		}
		if err := validateResourceSubtype(current.Kind, value); err != nil {
			return Resource{}, err
		}
		input.Subtype = &value
	}
	if input.Labels != nil {
		if err := validateLabels(*input.Labels); err != nil {
			return Resource{}, err
		}
		if *input.Labels == nil {
			value := map[string]string{}
			input.Labels = &value
		}
	}
	if input.Config != nil {
		if *input.Config == nil {
			value := map[string]any{}
			input.Config = &value
		}
		current, err := s.store.Get(ctx, id)
		if err != nil {
			return Resource{}, err
		}
		schema, err := s.store.GetSchema(ctx, current.Kind, current.SchemaVersion)
		if err != nil {
			return Resource{}, err
		}
		if err := validateConfig(*input.Config, schema); err != nil {
			return Resource{}, err
		}
		if current.Kind == "AIProvider" {
			if err := validateAIProviderConfig(*input.Config); err != nil {
				return Resource{}, err
			}
		}
		if current.Kind == "Workflow" {
			if err := validateWorkflowConfig(*input.Config); err != nil {
				return Resource{}, err
			}
		}
	}
	if input.ScopeID == nil && input.Name == nil && input.ExternalUID == nil && input.SourceResourceID == nil && input.Labels == nil && input.Config == nil && input.Status == nil && input.CredentialID == nil {
		return Resource{}, invalid("at least one field must be provided")
	}
	return s.store.Update(ctx, id, input)
}

func validateAIProviderConfig(config map[string]any) error {
	providerType, _ := config["provider_type"].(string)
	baseURL, _ := config["base_url"].(string)
	if strings.TrimSpace(providerType) == "" {
		return invalid("AIProvider provider_type is required")
	}
	if strings.TrimSpace(baseURL) == "" {
		return invalid("AIProvider base_url is required")
	}
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return invalid("AIProvider base_url must be an absolute HTTP URL")
	}
	models, ok := config["models"].([]any)
	if !ok || len(models) == 0 {
		return invalid("AIProvider models must contain at least one model")
	}
	seen := map[string]bool{}
	for index, raw := range models {
		model, ok := raw.(map[string]any)
		if !ok {
			return invalid(fmt.Sprintf("AIProvider models[%d] must be an object", index))
		}
		name, _ := model["name"].(string)
		if strings.TrimSpace(name) == "" {
			return invalid(fmt.Sprintf("AIProvider models[%d].name is required", index))
		}
		if seen[name] {
			return invalid("AIProvider model names must be unique")
		}
		seen[name] = true
		contextWindow, ok := configInt(model["context_window_tokens"])
		if !ok || contextWindow <= 0 {
			return invalid(fmt.Sprintf("AIProvider models[%d].context_window_tokens must be positive", index))
		}
		if temperature, exists := model["temperature"]; exists {
			value, ok := configFloat(temperature)
			if !ok || value < 0 || value > 2 {
				return invalid(fmt.Sprintf("AIProvider models[%d].temperature must be between 0 and 2", index))
			}
		}
		if enabled, exists := model["enabled"]; exists {
			if _, ok := enabled.(bool); !ok {
				return invalid(fmt.Sprintf("AIProvider models[%d].enabled must be boolean", index))
			}
		}
		if capabilities, exists := model["capabilities"]; exists {
			if _, ok := capabilities.([]any); !ok {
				return invalid(fmt.Sprintf("AIProvider models[%d].capabilities must be an array", index))
			}
		}
	}
	if defaultModel, exists := config["default_model"]; exists {
		name, _ := defaultModel.(string)
		if name != "" && !seen[name] {
			return invalid("AIProvider default_model must be declared in models")
		}
	}
	return nil
}

func configInt(value any) (int, bool) {
	switch number := value.(type) {
	case int:
		return number, true
	case int32:
		return int(number), true
	case int64:
		return int(number), true
	case float64:
		return int(number), number == float64(int(number))
	case float32:
		return int(number), number == float32(int(number))
	default:
		return 0, false
	}
}

func configFloat(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	default:
		return 0, false
	}
}

// validateWorkflowConfig keeps resource writes independent from the
// AIEngine package (which consumes resources for context tooling) while
// enforcing the same persisted DAG invariants at the catalog boundary.
func validateWorkflowConfig(config map[string]any) error {
	raw, err := json.Marshal(config)
	if err != nil {
		return invalid("Workflow config is invalid")
	}
	var parsed struct {
		Version int `json:"version"`
		Nodes   []struct {
			ID string `json:"id"`
		} `json:"nodes"`
		Edges []struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || parsed.Version < 1 || len(parsed.Nodes) == 0 {
		return invalid("Workflow config requires a positive version and at least one node")
	}
	seen := make(map[string]bool, len(parsed.Nodes))
	for _, node := range parsed.Nodes {
		if strings.TrimSpace(node.ID) == "" || seen[node.ID] {
			return invalid("Workflow node IDs must be non-empty and unique")
		}
		seen[node.ID] = true
	}
	graph := make(map[string][]string, len(seen))
	for _, edge := range parsed.Edges {
		if !seen[edge.From] || !seen[edge.To] || edge.From == edge.To {
			return invalid("Workflow edge references an invalid node")
		}
		graph[edge.From] = append(graph[edge.From], edge.To)
	}
	state := make(map[string]uint8, len(seen))
	var visit func(string) bool
	visit = func(id string) bool {
		if state[id] == 1 {
			return false
		}
		if state[id] == 2 {
			return true
		}
		state[id] = 1
		for _, child := range graph[id] {
			if !visit(child) {
				return false
			}
		}
		state[id] = 2
		return true
	}
	for id := range seen {
		if !visit(id) {
			return invalid("Workflow config contains a cycle")
		}
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return invalid("resource_id is required")
	}
	return s.store.Delete(ctx, id)
}

func (s *Service) ListSchemas(ctx context.Context) ([]Schema, error) { return s.store.ListSchemas(ctx) }

func (s *Service) CreateRelation(ctx context.Context, actorID string, input CreateRelationInput) (Relation, error) {
	input.SourceResourceID = strings.TrimSpace(input.SourceResourceID)
	input.TargetResourceID = strings.TrimSpace(input.TargetResourceID)
	input.RelationType = strings.TrimSpace(input.RelationType)
	if input.SourceResourceID == "" || input.TargetResourceID == "" || input.RelationType == "" {
		return Relation{}, invalid("source_resource_id, target_resource_id and relation_type are required")
	}
	if input.DiscoverySource == "" {
		input.DiscoverySource = "manual"
	}
	if input.Confidence == 0 {
		input.Confidence = 1
	}
	if input.Attributes == nil {
		input.Attributes = map[string]any{}
	}
	if input.Confidence < 0 || input.Confidence > 1 {
		return Relation{}, invalid("confidence must be between 0 and 1")
	}
	source, err := s.store.Get(ctx, input.SourceResourceID)
	if err != nil {
		return Relation{}, err
	}
	if !allowsResource(ctx, source.ScopeID, source.ID) {
		return Relation{}, authorization.ErrForbidden
	}
	return s.store.CreateRelation(ctx, input, actorID)
}

func (s *Service) ListRelations(ctx context.Context, resourceID string) ([]Relation, error) {
	if strings.TrimSpace(resourceID) == "" {
		return nil, invalid("resource_id is required")
	}
	return s.store.ListRelations(ctx, resourceID)
}

func (s *Service) DeleteRelation(ctx context.Context, resourceID, relationID string) error {
	if strings.TrimSpace(resourceID) == "" || strings.TrimSpace(relationID) == "" {
		return invalid("resource_id and relation_id are required")
	}
	source, err := s.store.Get(ctx, resourceID)
	if err != nil {
		return err
	}
	if !allowsResource(ctx, source.ScopeID, source.ID) {
		return authorization.ErrForbidden
	}
	return s.store.DeleteRelation(ctx, resourceID, relationID)
}

func (s *Service) Topology(ctx context.Context, resourceID string, depth, maxNodes int) ([]TopologyNode, error) {
	if strings.TrimSpace(resourceID) == "" {
		return nil, invalid("resource_id is required")
	}
	if depth == 0 {
		depth = 5
	}
	if maxNodes == 0 {
		maxNodes = 100
	}
	if depth < 1 || depth > 8 || maxNodes < 1 || maxNodes > 200 {
		return nil, invalid("depth must be 1-8 and max_nodes must be 1-200")
	}
	return s.store.Topology(ctx, resourceID, depth, maxNodes)
}

func (s *Service) SetDefault(ctx context.Context, scopeID, key, resourceID string) (Default, error) {
	if strings.TrimSpace(scopeID) == "" || strings.TrimSpace(key) == "" || strings.TrimSpace(resourceID) == "" {
		return Default{}, invalid("scope_id, default_key and resource_id are required")
	}
	if !allowsExactScope(ctx, scopeID) {
		return Default{}, authorization.ErrForbidden
	}
	return s.store.SetDefault(ctx, strings.TrimSpace(scopeID), strings.TrimSpace(key), strings.TrimSpace(resourceID))
}

func (s *Service) ResolveDefault(ctx context.Context, scopeID, key string) (Resource, error) {
	if strings.TrimSpace(scopeID) == "" || strings.TrimSpace(key) == "" {
		return Resource{}, invalid("scope_id and default_key are required")
	}
	if !allowsExactScope(ctx, scopeID) {
		return Resource{}, authorization.ErrForbidden
	}
	return s.store.ResolveDefault(ctx, strings.TrimSpace(scopeID), strings.TrimSpace(key))
}

func validateResourceInput(scopeID, kind, name string) error {
	if scopeID == "" {
		return invalid("scope_id is required")
	}
	if kind == "" || len([]rune(kind)) > 120 {
		return invalid("kind must contain 1 to 120 characters")
	}
	if name == "" || len([]rune(name)) > 200 {
		return invalid("name must contain 1 to 200 characters")
	}
	return nil
}

var directAgentKinds = map[string]struct{}{
	"Host": {}, "Docker": {}, "Kubernetes": {}, "Redis": {}, "TongRDS": {},
	"Kafka": {}, "RabbitMQ": {}, "Elasticsearch": {}, "OceanBase": {},
	"Oracle": {}, "MySQL": {}, "PostgreSQL": {},
}

func normalizeResourceSubtype(kind, subtype string) string {
	subtype = strings.TrimSpace(subtype)
	if _, ok := directAgentKinds[kind]; !ok {
		return subtype
	}
	if subtype == "" {
		return "Direct"
	}
	return subtype
}

func validateResourceSubtype(kind, subtype string) error {
	if _, ok := directAgentKinds[kind]; !ok {
		return nil
	}
	if subtype != "Direct" && subtype != "Agent" {
		return invalid(fmt.Sprintf("%s subtype must be Direct or Agent", kind))
	}
	return nil
}

func validateStatus(status string) error {
	if status != StatusActive && status != StatusDisabled && status != StatusUnknown {
		return invalid("status must be active, disabled or unknown")
	}
	return nil
}

func validateLabels(labels map[string]string) error {
	if len(labels) > 50 {
		return invalid("labels must contain at most 50 entries")
	}
	for key, value := range labels {
		if strings.TrimSpace(key) == "" || len([]rune(key)) > 100 || len([]rune(value)) > 500 {
			return invalid("labels contain an invalid key or value")
		}
	}
	return nil
}

func normalizePagination(pagination Pagination) (Pagination, error) {
	if pagination.Page == 0 {
		pagination.Page = 1
	}
	if pagination.PageSize == 0 {
		pagination.PageSize = 50
	}
	if pagination.Page < 1 || pagination.PageSize < 1 || pagination.PageSize > 200 {
		return Pagination{}, invalid("page must be positive and page_size must be between 1 and 200")
	}
	return pagination, nil
}

func allowsExactScope(ctx context.Context, scopeID string) bool {
	filter, restricted := authorization.ScopeFilterFromContext(ctx)
	return !restricted || filter.Allows(scopeID)
}

func allowsResource(ctx context.Context, scopeID, resourceID string) bool {
	if filter, restricted := authorization.ResourceFilterFromContext(ctx); restricted {
		return filter.Allows(scopeID, resourceID)
	}
	return allowsExactScope(ctx, scopeID)
}
