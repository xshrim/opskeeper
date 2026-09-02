package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/openaimodel"
	"google.golang.org/genai"
	"opskeeper/backend/authorization"
	"opskeeper/backend/resource"
)

type ResourceReader interface {
	Get(context.Context, string) (resource.Resource, error)
}
type ResourceLister interface {
	List(context.Context, resource.Pagination, string, map[string]string) (resource.Page[resource.Resource], error)
}
type CredentialReader interface {
	RevealLinked(context.Context, string) ([]byte, error)
}
type Service struct {
	store       Store
	resources   ResourceReader
	credentials CredentialReader
}

type AvailableModel struct {
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
}
type AvailableProvider struct {
	ResourceID string           `json:"provider_resource_id"`
	Name       string           `json:"name"`
	Models     []AvailableModel `json:"models"`
	Default    bool             `json:"default"`
}

func (s *Service) Available(ctx context.Context, scopeID string, purpose Purpose) ([]AvailableProvider, error) {
	lister, ok := s.resources.(ResourceLister)
	if !ok {
		return nil, fmt.Errorf("resource listing is unavailable")
	}
	page, err := lister.List(ctx, resource.Pagination{Page: 1, PageSize: 100}, AIProviderKind, nil)
	if err != nil {
		return nil, err
	}
	binding, _ := s.store.ResolveBinding(ctx, scopeID, purpose)
	items := make([]AvailableProvider, 0, len(page.Items))
	for _, item := range page.Items {
		if item.Status != resource.StatusActive || !allowsProviderResource(ctx, item) {
			continue
		}
		provider, err := s.readAIProvider(ctx, item.ID, true)
		if err != nil {
			continue
		}
		models := make([]AvailableModel, 0)
		for _, model := range provider.Config.Models {
			if model.Enabled && len(missingCapabilities(model, requiredCapabilities(purpose))) == 0 {
				models = append(models, AvailableModel{Name: model.Name, Capabilities: model.Capabilities})
			}
		}
		if len(models) > 0 {
			items = append(items, AvailableProvider{ResourceID: item.ID, Name: item.Name, Models: models, Default: binding.ProviderID == item.ID})
		}
	}
	return items, nil
}

func NewService(store Store, resources ResourceReader, credentials CredentialReader) *Service {
	return &Service{store: store, resources: resources, credentials: credentials}
}

func (s *Service) AIProvider(ctx context.Context, id string) (AIProvider, error) {
	return s.readAIProvider(ctx, strings.TrimSpace(id), false)
}

func (s *Service) ListBindings(ctx context.Context, scopeID string) ([]ScopeProviderBinding, error) {
	if strings.TrimSpace(scopeID) == "" || !allowsScope(ctx, scopeID) {
		return nil, authorization.ErrForbidden
	}
	return s.store.ListBindings(ctx, strings.TrimSpace(scopeID))
}

func (s *Service) SetBinding(ctx context.Context, actorID, scopeID string, purpose Purpose, providerID string) (ScopeProviderBinding, error) {
	scopeID, providerID = strings.TrimSpace(scopeID), strings.TrimSpace(providerID)
	if scopeID == "" || providerID == "" || !validPurpose(purpose) {
		return ScopeProviderBinding{}, invalid("scope_id, purpose and provider_resource_id are required")
	}
	if !allowsExactScope(ctx, scopeID) {
		return ScopeProviderBinding{}, authorization.ErrForbidden
	}
	provider, err := s.readAIProvider(ctx, providerID, true)
	if err != nil {
		return ScopeProviderBinding{}, err
	}
	model, ok := selectedDefaultModel(provider.Config)
	if !ok {
		return ScopeProviderBinding{}, invalid("AIProvider default_model must reference an enabled model")
	}
	if missing := missingCapabilities(model, requiredCapabilities(purpose)); len(missing) > 0 {
		return ScopeProviderBinding{}, invalid("default model lacks capabilities: " + strings.Join(missing, ", "))
	}
	return s.store.SetBinding(ctx, ScopeProviderBinding{ScopeID: scopeID, ProviderID: providerID, Tag: purpose}, strings.TrimSpace(actorID))
}

func (s *Service) RemoveBinding(ctx context.Context, scopeID string, purpose Purpose) error {
	if strings.TrimSpace(scopeID) == "" || !validPurpose(purpose) || !allowsExactScope(ctx, scopeID) {
		return authorization.ErrForbidden
	}
	return s.store.RemoveBinding(ctx, strings.TrimSpace(scopeID), purpose)
}

// Resolve chooses a Provider directly. A blank provider ID resolves the
// nearest Scope binding for the requested role, falling back to
// the general role at each scope before moving to its parent.
func (s *Service) Resolve(ctx context.Context, scopeID, providerID, modelName string, purpose Purpose) (ResolvedProvider, error) {
	scopeID = strings.TrimSpace(scopeID)
	if scopeID == "" {
		return ResolvedProvider{}, authorization.ErrForbidden
	}
	providerID, modelName = strings.TrimSpace(providerID), strings.TrimSpace(modelName)
	definedAt, reason := scopeID, "explicit_provider"
	if providerID == "" {
		if !allowsScope(ctx, scopeID) {
			return ResolvedProvider{}, authorization.ErrForbidden
		}
		if !validPurpose(purpose) {
			return ResolvedProvider{}, invalid("purpose is not supported")
		}
		binding, err := s.store.ResolveBinding(ctx, scopeID, purpose)
		if err != nil {
			return ResolvedProvider{}, invalid("no AIProvider is configured for role " + string(purpose))
		}
		providerID, definedAt, reason = binding.ProviderID, binding.ScopeID, "scope_purpose_"+string(binding.Tag)
	}
	provider, err := s.readAIProvider(ctx, providerID, true)
	if err != nil {
		return ResolvedProvider{}, err
	}
	if modelName == "" {
		modelName = provider.Config.DefaultModel
		if modelName == "" {
			for _, candidate := range provider.Config.Models {
				if candidate.Enabled {
					modelName = candidate.Name
					break
				}
			}
		}
	}
	selected, ok := findEnabledModel(provider.Config.Models, modelName)
	if !ok {
		return ResolvedProvider{}, invalid("model_name is not an enabled model of the AIProvider")
	}
	if missing := missingCapabilities(selected, requiredCapabilities(purpose)); len(missing) > 0 {
		return ResolvedProvider{}, invalid("selected model lacks capabilities: " + strings.Join(missing, ", "))
	}
	result := ResolvedProvider{ProviderResourceID: provider.ResourceID, ProviderName: provider.Name, Provider: provider, Model: selected, DefinedAtScopeID: definedAt, SelectionReason: reason}
	return s.attachCredential(ctx, result)
}

func (s *Service) BuildModel(ctx context.Context, scopeID, providerID, modelName string, purpose Purpose) (ResolvedProvider, model.LLM, error) {
	resolved, err := s.Resolve(ctx, scopeID, providerID, modelName, purpose)
	if err != nil {
		return ResolvedProvider{}, nil, err
	}
	if resolved.Provider.Config.ProviderType == "openai" {
		client, err := openaimodel.NewModel(ctx, resolved.Model.Name, &openaimodel.ClientConfig{APIKey: resolved.APIKey, BaseURL: resolved.Provider.Config.BaseURL})
		return resolved, client, err
	}
	client, err := NewChatCompletionsModel(ChatCompletionsConfig{APIKey: resolved.APIKey, BaseURL: resolved.Provider.Config.BaseURL, ModelName: resolved.Model.Name})
	return resolved, client, err
}

func (s *Service) TestConnection(ctx context.Context, scopeID, providerID, modelName string, stream bool) (ConnectionResult, error) {
	started := time.Now()
	resolved, client, err := s.BuildModel(ctx, scopeID, providerID, modelName, PurposeGeneral)
	if err != nil {
		return ConnectionResult{}, err
	}
	text, err := runProbe(ctx, client, resolved.Model.Name, stream)
	if err != nil {
		return ConnectionResult{}, err
	}
	if strings.TrimSpace(text) == "" {
		return ConnectionResult{}, fmt.Errorf("AIProvider connection returned no text")
	}
	return ConnectionResult{ProviderResourceID: resolved.Provider.ResourceID, ModelName: resolved.Model.Name, Status: "succeeded", LatencyMS: time.Since(started).Milliseconds(), Message: "模型连接测试通过"}, nil
}

func (s *Service) TestDraftConnection(ctx context.Context, draft DraftConnection, stream bool) (ConnectionResult, error) {
	started := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	draft.ScopeID, draft.ProviderType, draft.BaseURL, draft.ModelName = strings.TrimSpace(draft.ScopeID), strings.TrimSpace(draft.ProviderType), strings.TrimSpace(draft.BaseURL), strings.TrimSpace(draft.ModelName)
	if draft.ScopeID == "" || !allowsScope(ctx, draft.ScopeID) {
		return ConnectionResult{}, authorization.ErrForbidden
	}
	config := AIProviderConfig{ProviderType: draft.ProviderType, BaseURL: draft.BaseURL, Enabled: true, Models: []ProviderModel{{Name: draft.ModelName, ContextWindowTokens: draft.ContextWindow, Temperature: draft.Temperature, Capabilities: draft.Capabilities, Enabled: true}}, DefaultModel: draft.ModelName, TimeoutSeconds: 60}
	if err := validateAIProviderConfig(config); err != nil {
		return ConnectionResult{}, err
	}
	var client model.LLM
	var err error
	if config.ProviderType == "openai" {
		client, err = openaimodel.NewModel(ctx, draft.ModelName, &openaimodel.ClientConfig{APIKey: draft.APIKey, BaseURL: draft.BaseURL})
	} else {
		client, err = NewChatCompletionsModel(ChatCompletionsConfig{APIKey: draft.APIKey, BaseURL: draft.BaseURL, ModelName: draft.ModelName})
	}
	if err != nil {
		return ConnectionResult{}, err
	}
	if _, err := runProbe(ctx, client, draft.ModelName, stream); err != nil {
		return ConnectionResult{}, err
	}
	return ConnectionResult{ModelName: draft.ModelName, Status: "succeeded", LatencyMS: time.Since(started).Milliseconds(), Message: "模型连接测试通过"}, nil
}

func (s *Service) attachCredential(ctx context.Context, result ResolvedProvider) (ResolvedProvider, error) {
	if result.Provider.CredentialID == "" {
		return result, nil
	}
	if s.credentials == nil {
		return ResolvedProvider{}, fmt.Errorf("credential service is unavailable")
	}
	secret, err := s.credentials.RevealLinked(ctx, result.Provider.CredentialID)
	if err != nil {
		return ResolvedProvider{}, fmt.Errorf("read AIProvider credential: %w", err)
	}
	result.APIKey = apiKeyFromCredential(secret)
	return result, nil
}

func apiKeyFromCredential(secret []byte) string {
	var fields map[string]string
	if json.Unmarshal(secret, &fields) == nil {
		return strings.TrimSpace(fields["token"])
	}
	return strings.TrimSpace(string(secret))
}

func runProbe(ctx context.Context, client model.LLM, modelName string, stream bool) (string, error) {
	request := &model.LLMRequest{Model: modelName, Contents: []*genai.Content{genai.NewContentFromText("Reply with OK only.", genai.RoleUser)}}
	var out strings.Builder
	for response, err := range client.GenerateContent(ctx, request, stream) {
		if err != nil {
			return "", err
		}
		if response != nil && response.Content != nil {
			for _, part := range response.Content.Parts {
				if part != nil {
					out.WriteString(part.Text)
				}
			}
		}
	}
	return out.String(), nil
}

func (s *Service) readAIProvider(ctx context.Context, id string, requireActive bool) (AIProvider, error) {
	if id == "" || s.resources == nil {
		return AIProvider{}, invalid("ai_provider_resource_id is required")
	}
	item, err := s.resources.Get(ctx, id)
	if err != nil {
		return AIProvider{}, err
	}
	if item.Kind != AIProviderKind {
		return AIProvider{}, invalid("resource is not an AIProvider")
	}
	if !allowsProviderResource(ctx, item) {
		return AIProvider{}, authorization.ErrForbidden
	}
	if requireActive && (item.Status != resource.StatusActive) {
		return AIProvider{}, invalid("AIProvider is not active")
	}
	encoded, err := json.Marshal(item.Config)
	if err != nil {
		return AIProvider{}, fmt.Errorf("encode AIProvider config: %w", err)
	}
	var config AIProviderConfig
	if err := json.Unmarshal(encoded, &config); err != nil {
		return AIProvider{}, invalid("AIProvider config is invalid")
	}
	for index := range config.Models {
		if config.Models[index].MaxOutputTokens <= 0 {
			config.Models[index].MaxOutputTokens = 128000
		}
	}
	if err := validateAIProviderConfig(config); err != nil {
		return AIProvider{}, err
	}
	if requireActive && !config.Enabled {
		return AIProvider{}, invalid("AIProvider is disabled")
	}
	credentialID := ""
	if item.CredentialID != nil {
		credentialID = *item.CredentialID
	}
	return AIProvider{ResourceID: item.ID, ScopeID: item.ScopeID, Name: item.Name, CredentialID: credentialID, Config: config}, nil
}

func validateAIProviderConfig(config AIProviderConfig) error {
	if !supportedProviderType(config.ProviderType) {
		return invalid("provider_type is not supported")
	}
	parsed, err := url.ParseRequestURI(config.BaseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return invalid("base_url must be an absolute HTTP URL")
	}
	if len(config.Models) == 0 || len(config.Models) > 100 {
		return invalid("models must contain 1 to 100 entries")
	}
	names := make([]string, 0, len(config.Models))
	for _, item := range config.Models {
		if strings.TrimSpace(item.Name) == "" || item.ContextWindowTokens <= 0 || item.Temperature < 0 || item.Temperature > 2 {
			return invalid("every provider model requires a name and positive context_window_tokens")
		}
		if item.MaxOutputTokens <= 0 {
			item.MaxOutputTokens = 128000
		}
		if slices.Contains(names, item.Name) {
			return invalid("provider model names must be unique")
		}
		names = append(names, item.Name)
	}
	if config.DefaultModel != "" {
		selected, ok := findModel(config.Models, config.DefaultModel)
		if !ok || !selected.Enabled {
			return invalid("default_model must reference an enabled model")
		}
	}
	if config.TimeoutSeconds < 0 || config.TimeoutSeconds > int((5*time.Minute).Seconds()) {
		return invalid("timeout_seconds must not exceed 300")
	}
	return nil
}

func supportedProviderType(provider string) bool {
	switch strings.TrimSpace(provider) {
	case "openai_compatible", "openai", "anthropic", "gemini", "grok", "deepseek", "qwen", "kimi", "glm", "minimax", "mimo", "longcat", "doubao", "openrouter", "siliconflow", "ollama":
		return true
	default:
		return false
	}
}
func findModel(models []ProviderModel, name string) (ProviderModel, bool) {
	for _, item := range models {
		if item.Name == name {
			return item, true
		}
	}
	return ProviderModel{}, false
}
func findEnabledModel(models []ProviderModel, name string) (ProviderModel, bool) {
	if name != "" {
		item, ok := findModel(models, name)
		return item, ok && item.Enabled
	}
	for _, item := range models {
		if item.Enabled {
			return item, true
		}
	}
	return ProviderModel{}, false
}
func selectedDefaultModel(config AIProviderConfig) (ProviderModel, bool) {
	return findEnabledModel(config.Models, config.DefaultModel)
}
func validPurpose(purpose Purpose) bool {
	switch purpose {
	case PurposeGeneral, PurposeDiagnosis, PurposeInspection, PurposeWorkflow:
		return true
	default:
		return false
	}
}
func requiredCapabilities(purpose Purpose) []string {
	switch purpose {
	case PurposeDiagnosis:
		return []string{"text", "tool_calling", "stream"}
	case PurposeInspection, PurposeWorkflow:
		return []string{"text", "tool_calling", "structured_output"}
	default:
		return []string{"text"}
	}
}
func missingCapabilities(model ProviderModel, required []string) []string {
	missing := make([]string, 0)
	for _, capability := range required {
		if !slices.Contains(model.Capabilities, capability) {
			missing = append(missing, capability)
		}
	}
	return missing
}
func allowsScope(ctx context.Context, scopeID string) bool {
	filter, ok := authorization.ScopeFilterFromContext(ctx)
	return !ok || filter.Allows(scopeID)
}
func allowsExactScope(ctx context.Context, scopeID string) bool { return allowsScope(ctx, scopeID) }
func allowsProviderResource(ctx context.Context, item resource.Resource) bool {
	if filter, ok := authorization.ResourceFilterFromContext(ctx); ok {
		return filter.Allows(item.ScopeID, item.ID)
	}
	return allowsScope(ctx, item.ScopeID)
}
