package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/openaimodel"
	"google.golang.org/genai"
	"opskeeper/backend/authorization"
)

type Service struct {
	store     Store
	providers ProviderReader
}

var ProviderTags = []Purpose{PurposeGeneral, PurposeDiagnosis, PurposeInspection, PurposeWorkflow}
var ModelTags = []string{"text", "stream", "tool_calling", "structured_output", "reasoning", "embedding", "audio", "vision", "image_generation"}

type AvailableModel struct {
	Name                string   `json:"name"`
	ContextWindowTokens int      `json:"context_window_tokens"`
	Capabilities        []string `json:"capabilities"`
}
type AvailableProvider struct {
	ID      string           `json:"provider_id"`
	Name    string           `json:"name"`
	Models  []AvailableModel `json:"models"`
	Default bool             `json:"default"`
}

func (s *Service) Available(ctx context.Context, scopeID string, purpose Purpose) ([]AvailableProvider, error) {
	if s.providers != nil {
		providers, err := s.providers.ListProviders(ctx, scopeID)
		if err != nil {
			return nil, err
		}
		binding, _ := s.store.ResolveBinding(ctx, scopeID, purpose)
		items := make([]AvailableProvider, 0, len(providers))
		for _, provider := range providers {
			if !allowsScope(ctx, provider.ScopeID) || !provider.Config.Enabled {
				continue
			}
			models := make([]AvailableModel, 0)
			for _, model := range provider.Config.Models {
				if model.Enabled && len(missingCapabilities(model, requiredCapabilities(purpose))) == 0 {
					models = append(models, AvailableModel{Name: model.Name, ContextWindowTokens: model.ContextWindowTokens, Capabilities: model.Capabilities})
				}
			}
			if len(models) > 0 {
				items = append(items, AvailableProvider{ID: provider.ID, Name: provider.Name, Models: models, Default: binding.ProviderID == provider.ID})
			}
		}
		return items, nil
	}
	return nil, fmt.Errorf("provider catalog is unavailable")
}

func NewService(store Store, providers ProviderReader) *Service {
	return &Service{store: store, providers: providers}
}

func (s *Service) ListProviders(ctx context.Context, scopeID string) ([]Provider, error) {
	if s.providers == nil {
		return nil, fmt.Errorf("provider catalog is unavailable")
	}
	if !allowsScope(ctx, strings.TrimSpace(scopeID)) {
		return nil, authorization.ErrForbidden
	}
	return s.providers.ListProviders(ctx, strings.TrimSpace(scopeID))
}

func (s *Service) CreateProvider(ctx context.Context, input ProviderInput) (Provider, error) {
	writer, ok := s.providers.(ProviderCatalogWriter)
	if !ok {
		return Provider{}, fmt.Errorf("provider catalog is read-only")
	}
	input.ScopeID, input.Name, input.Status = strings.TrimSpace(input.ScopeID), strings.TrimSpace(input.Name), strings.TrimSpace(input.Status)
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Name == "" || !allowsExactScope(ctx, input.ScopeID) {
		return Provider{}, authorization.ErrForbidden
	}
	normalizeProviderInput(&input.Config)
	if err := validateProviderConfig(input.Config); err != nil {
		return Provider{}, err
	}
	return writer.CreateProvider(ctx, input)
}

func (s *Service) UpdateProvider(ctx context.Context, id string, patch ProviderPatch) (Provider, error) {
	writer, ok := s.providers.(ProviderCatalogWriter)
	if !ok {
		return Provider{}, fmt.Errorf("provider catalog is read-only")
	}
	current, err := s.readProvider(ctx, strings.TrimSpace(id), false)
	if err != nil {
		return Provider{}, err
	}
	if !allowsExactScope(ctx, current.ScopeID) {
		return Provider{}, authorization.ErrForbidden
	}
	if patch.Name != nil {
		value := strings.TrimSpace(*patch.Name)
		if value == "" {
			return Provider{}, invalid("name is required")
		}
		patch.Name = &value
	}
	if patch.Config != nil {
		normalizeProviderInput(patch.Config)
		if err := validateProviderConfig(*patch.Config); err != nil {
			return Provider{}, err
		}
	}
	return writer.UpdateProvider(ctx, id, patch)
}

func (s *Service) DeleteProvider(ctx context.Context, id string) error {
	writer, ok := s.providers.(ProviderCatalogWriter)
	if !ok {
		return fmt.Errorf("provider catalog is read-only")
	}
	current, err := s.readProvider(ctx, strings.TrimSpace(id), false)
	if err != nil {
		return err
	}
	if !allowsExactScope(ctx, current.ScopeID) {
		return authorization.ErrForbidden
	}
	return writer.DeleteProvider(ctx, current.ID)
}

func normalizeProviderInput(config *ProviderConfig) {
	if config.Icon == "" {
		config.Icon = "lucide:Bot"
	}
	for index := range config.Models {
		model := &config.Models[index]
		model.Tags = normalizeModelTags(model.Tags)
		model.Capabilities = normalizeModelTags(model.Capabilities)
		if len(model.Tags) == 0 {
			model.Tags = append([]string(nil), model.Capabilities...)
		}
		if len(model.Capabilities) == 0 {
			model.Capabilities = append([]string(nil), model.Tags...)
		}
	}
}

func (s *Service) Provider(ctx context.Context, id string) (Provider, error) {
	return s.readProvider(ctx, strings.TrimSpace(id), false)
}

func (s *Service) RecordConnectionTest(ctx context.Context, providerID, status, message string, latencyMS int64, checkedAt time.Time) error {
	recorder, ok := s.providers.(ProviderConnectionTestWriter)
	if !ok {
		return fmt.Errorf("provider connection test writer is unavailable")
	}
	return recorder.RecordConnectionTest(ctx, providerID, status, message, latencyMS, checkedAt)
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
		return ScopeProviderBinding{}, invalid("scope_id, purpose and provider_id are required")
	}
	if !allowsExactScope(ctx, scopeID) {
		return ScopeProviderBinding{}, authorization.ErrForbidden
	}
	provider, err := s.providers.GetProvider(ctx, providerID)
	if err != nil {
		return ScopeProviderBinding{}, err
	}
	if provider.Status != "active" || !provider.Config.Enabled {
		return ScopeProviderBinding{}, invalid("Provider is disabled")
	}
	normalizeProviderConfig(&provider.Config)
	if err := validateProviderConfig(provider.Config); err != nil {
		return ScopeProviderBinding{}, err
	}
	model, ok := selectedDefaultModel(provider.Config)
	if !ok {
		return ScopeProviderBinding{}, invalid("Provider default_model must reference an enabled model")
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
			return ResolvedProvider{}, invalid("no Provider is configured for role " + string(purpose))
		}
		providerID, definedAt, reason = binding.ProviderID, binding.ScopeID, "scope_purpose_"+string(binding.Tag)
	}
	provider, err := s.readProvider(ctx, providerID, true)
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
		return ResolvedProvider{}, invalid("model_name is not an enabled model of the Provider")
	}
	if missing := missingCapabilities(selected, requiredCapabilities(purpose)); len(missing) > 0 {
		return ResolvedProvider{}, invalid("selected model lacks capabilities: " + strings.Join(missing, ", "))
	}
	result := ResolvedProvider{ProviderID: provider.ID, ProviderName: provider.Name, Provider: provider, Model: selected, DefinedAtScopeID: definedAt, SelectionReason: reason}
	return s.attachCredential(ctx, result)
}

func (s *Service) BuildModel(ctx context.Context, scopeID, providerID, modelName string, purpose Purpose) (ResolvedProvider, model.LLM, error) {
	resolved, err := s.Resolve(ctx, scopeID, providerID, modelName, purpose)
	if err != nil {
		return ResolvedProvider{}, nil, err
	}
	if resolved.Provider.Config.ProviderType == "openai" {
		client, err := openaimodel.NewModel(ctx, resolved.Model.Name, &openaimodel.ClientConfig{APIKey: resolved.APIKey, BaseURL: resolved.Provider.Config.BaseURL, HTTPClient: providerHTTPClient(resolved.Provider.Config.TimeoutSeconds)})
		return resolved, client, err
	}
	client, err := NewChatCompletionsModel(ChatCompletionsConfig{APIKey: resolved.APIKey, BaseURL: resolved.Provider.Config.BaseURL, ModelName: resolved.Model.Name, HTTPClient: providerHTTPClient(resolved.Provider.Config.TimeoutSeconds)})
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
		return ConnectionResult{}, fmt.Errorf("Provider connection returned no text")
	}
	return ConnectionResult{ProviderID: resolved.Provider.ID, ModelName: resolved.Model.Name, Status: "succeeded", LatencyMS: time.Since(started).Milliseconds(), Message: "模型连接测试通过"}, nil
}

func (s *Service) TestDraftConnection(ctx context.Context, draft DraftConnection, stream bool) (ConnectionResult, error) {
	started := time.Now()
	timeout := providerTimeout(draft.TimeoutSeconds)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	draft.ScopeID, draft.ProviderType, draft.BaseURL, draft.ModelName = strings.TrimSpace(draft.ScopeID), strings.TrimSpace(draft.ProviderType), strings.TrimSpace(draft.BaseURL), strings.TrimSpace(draft.ModelName)
	if draft.ScopeID == "" || !allowsScope(ctx, draft.ScopeID) {
		return ConnectionResult{}, authorization.ErrForbidden
	}
	config := ProviderConfig{ProviderType: draft.ProviderType, BaseURL: draft.BaseURL, Enabled: true, Models: []ProviderModel{{Name: draft.ModelName, ContextWindowTokens: draft.ContextWindow, Temperature: draft.Temperature, Capabilities: draft.Capabilities, Enabled: true}}, DefaultModel: draft.ModelName, TimeoutSeconds: int(timeout / time.Second)}
	if err := validateProviderConfig(config); err != nil {
		return ConnectionResult{}, err
	}
	var client model.LLM
	var err error
	if config.ProviderType == "openai" {
		client, err = openaimodel.NewModel(ctx, draft.ModelName, &openaimodel.ClientConfig{APIKey: draft.APIKey, BaseURL: draft.BaseURL, HTTPClient: providerHTTPClient(int(timeout / time.Second))})
	} else {
		client, err = NewChatCompletionsModel(ChatCompletionsConfig{APIKey: draft.APIKey, BaseURL: draft.BaseURL, ModelName: draft.ModelName, HTTPClient: providerHTTPClient(int(timeout / time.Second))})
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
	secret, err := s.providers.RevealProviderSecret(ctx, result.Provider.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return result, nil
		}
		return ResolvedProvider{}, fmt.Errorf("read provider secret: %w", err)
	}
	result.APIKey = apiKeyFromSecret(secret)
	return result, nil
}

func apiKeyFromSecret(secret []byte) string {
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

func (s *Service) readProvider(ctx context.Context, id string, requireActive bool) (Provider, error) {
	if s.providers == nil || id == "" {
		return Provider{}, invalid("provider_id is required")
	}
	item, err := s.providers.GetProvider(ctx, id)
	if err != nil {
		return Provider{}, err
	}
	if !allowsScope(ctx, item.ScopeID) {
		return Provider{}, authorization.ErrForbidden
	}
	if err := validateProviderConfig(item.Config); err != nil {
		return Provider{}, err
	}
	if requireActive && (item.Status != "active" || !item.Config.Enabled) {
		return Provider{}, invalid("Provider is disabled")
	}
	return item, nil
}

func validateProviderConfig(config ProviderConfig) error {
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
		for _, tag := range item.Tags {
			if !slices.Contains(ModelTags, tag) {
				return invalid("unsupported provider model tag: " + tag)
			}
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
func selectedDefaultModel(config ProviderConfig) (ProviderModel, bool) {
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
