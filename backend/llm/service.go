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

type CredentialReader interface {
	RevealLinked(context.Context, string) ([]byte, error)
}

type Service struct {
	store       Store
	resources   ResourceReader
	credentials CredentialReader
}

func NewService(store Store, resources ResourceReader, credentials CredentialReader) *Service {
	return &Service{store: store, resources: resources, credentials: credentials}
}

func (s *Service) SetDefault(ctx context.Context, actorID, scopeID, providerID, modelName string) (Default, error) {
	scopeID = strings.TrimSpace(scopeID)
	providerID = strings.TrimSpace(providerID)
	modelName = strings.TrimSpace(modelName)
	if scopeID == "" || providerID == "" || modelName == "" {
		return Default{}, invalid("scope_id, provider_resource_id and model_name are required")
	}
	if !allowsScope(ctx, scopeID) {
		return Default{}, authorization.ErrForbidden
	}
	provider, err := s.readProvider(ctx, providerID, false)
	if err != nil {
		return Default{}, err
	}
	if _, ok := findModel(provider.Config.Models, modelName); !ok {
		return Default{}, invalid("model_name is not declared by the provider")
	}
	return s.store.SetDefault(ctx, Default{ScopeID: scopeID, ProviderResourceID: providerID, ModelName: modelName}, strings.TrimSpace(actorID))
}

func (s *Service) Resolve(ctx context.Context, scopeID, explicitProviderID, explicitModel string) (ResolvedModel, error) {
	scopeID = strings.TrimSpace(scopeID)
	if scopeID == "" {
		return ResolvedModel{}, authorization.ErrForbidden
	}
	definedAt := scopeID
	providerID := strings.TrimSpace(explicitProviderID)
	modelName := strings.TrimSpace(explicitModel)
	if providerID == "" {
		if !allowsScope(ctx, scopeID) {
			return ResolvedModel{}, authorization.ErrForbidden
		}
		binding, err := s.store.ResolveDefault(ctx, scopeID)
		if err != nil {
			return ResolvedModel{}, err
		}
		providerID, modelName, definedAt = binding.ProviderResourceID, binding.ModelName, binding.ScopeID
	}
	provider, err := s.readProvider(ctx, providerID, true)
	if err != nil {
		return ResolvedModel{}, err
	}
	if modelName == "" && len(provider.Config.Models) == 1 {
		modelName = provider.Config.Models[0].Name
	}
	modelConfig, ok := findModel(provider.Config.Models, modelName)
	if !ok {
		return ResolvedModel{}, invalid("model_name is not declared by the provider")
	}
	result := ResolvedModel{Provider: provider, Model: modelConfig, DefinedAtScopeID: definedAt}
	if provider.CredentialID != "" {
		if s.credentials == nil {
			return ResolvedModel{}, fmt.Errorf("credential service is unavailable")
		}
		secret, err := s.credentials.RevealLinked(ctx, provider.CredentialID)
		if err != nil {
			return ResolvedModel{}, fmt.Errorf("read LLM credential: %w", err)
		}
		result.APIKey = apiKeyFromCredential(secret)
	}
	return result, nil
}

func apiKeyFromCredential(secret []byte) string {
	var fields map[string]string
	if json.Unmarshal(secret, &fields) == nil {
		return strings.TrimSpace(fields["token"])
	}
	// Keep compatibility with credentials created before resource schemas
	// declared the token field as structured sensitive data.
	return strings.TrimSpace(string(secret))
}

func (s *Service) Provider(ctx context.Context, providerID string) (Provider, error) {
	return s.readProvider(ctx, strings.TrimSpace(providerID), false)
}

func (s *Service) BuildModel(ctx context.Context, scopeID, providerID, modelName string) (ResolvedModel, model.LLM, error) {
	resolved, err := s.Resolve(ctx, scopeID, providerID, modelName)
	if err != nil {
		return ResolvedModel{}, nil, err
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
	resolved, client, err := s.BuildModel(ctx, scopeID, providerID, modelName)
	if err != nil {
		return ConnectionResult{}, err
	}
	request := &model.LLMRequest{Model: resolved.Model.Name, Contents: []*genai.Content{genai.NewContentFromText("Reply with OK only.", genai.RoleUser)}}
	var responseText strings.Builder
	for response, generateErr := range client.GenerateContent(ctx, request, stream) {
		if generateErr != nil {
			return ConnectionResult{}, generateErr
		}
		if response != nil && response.Content != nil {
			for _, part := range response.Content.Parts {
				if part != nil {
					responseText.WriteString(part.Text)
				}
			}
		}
	}
	if strings.TrimSpace(responseText.String()) == "" {
		return ConnectionResult{}, fmt.Errorf("LLM connection returned no text")
	}
	return ConnectionResult{ProviderResourceID: resolved.Provider.ResourceID, ModelName: resolved.Model.Name, Status: "succeeded", LatencyMS: time.Since(started).Milliseconds(), Message: "模型连接测试通过"}, nil
}

func (s *Service) TestDraftConnection(ctx context.Context, draft DraftConnection, stream bool) (ConnectionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	draft.ScopeID = strings.TrimSpace(draft.ScopeID)
	draft.ProviderType = strings.TrimSpace(draft.ProviderType)
	draft.BaseURL = strings.TrimSpace(draft.BaseURL)
	draft.ModelName = strings.TrimSpace(draft.ModelName)
	if draft.ScopeID == "" || !allowsScope(ctx, draft.ScopeID) {
		return ConnectionResult{}, authorization.ErrForbidden
	}
	config := ProviderConfig{
		ProviderType:   draft.ProviderType,
		BaseURL:        draft.BaseURL,
		Models:         []ModelConfig{{Name: draft.ModelName, ContextWindow: draft.ContextWindow, Temperature: draft.Temperature, Capabilities: draft.Capabilities}},
		TimeoutSeconds: 60,
	}
	if err := validateProviderConfig(config); err != nil {
		return ConnectionResult{}, err
	}
	started := time.Now()
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
	temperature := float32(draft.Temperature)
	request := &model.LLMRequest{Model: draft.ModelName, Config: &genai.GenerateContentConfig{Temperature: &temperature}, Contents: []*genai.Content{genai.NewContentFromText("Reply with OK only.", genai.RoleUser)}}
	var responseText strings.Builder
	for response, generateErr := range client.GenerateContent(ctx, request, stream) {
		if generateErr != nil {
			return ConnectionResult{}, generateErr
		}
		if response != nil && response.Content != nil {
			for _, part := range response.Content.Parts {
				if part != nil {
					responseText.WriteString(part.Text)
				}
			}
		}
	}
	if strings.TrimSpace(responseText.String()) == "" {
		return ConnectionResult{}, fmt.Errorf("LLM connection returned no text")
	}
	return ConnectionResult{ModelName: draft.ModelName, Status: "succeeded", LatencyMS: time.Since(started).Milliseconds(), Message: "模型连接测试通过"}, nil
}

func (s *Service) readProvider(ctx context.Context, providerID string, requireActive bool) (Provider, error) {
	if providerID == "" || s.resources == nil {
		return Provider{}, invalid("provider_resource_id is required")
	}
	item, err := s.resources.Get(ctx, providerID)
	if err != nil {
		return Provider{}, err
	}
	if item.Kind != ProviderKind && item.Kind != AIEngineKind {
		return Provider{}, invalid("resource is not an AIEngine or LLMProvider")
	}
	if !allowsProviderResource(ctx, item) {
		return Provider{}, authorization.ErrForbidden
	}
	if requireActive && item.Status != resource.StatusActive {
		return Provider{}, invalid("AIEngine is not active")
	}
	encoded, err := json.Marshal(item.Config)
	if err != nil {
		return Provider{}, fmt.Errorf("encode LLMProvider config: %w", err)
	}
	var config ProviderConfig
	if item.Kind == AIEngineKind {
		var engine struct {
			Endpoints []struct {
				ProviderType   string   `json:"provider_type"`
				BaseURL        string   `json:"base_url"`
				ModelName      string   `json:"model_name"`
				ContextWindow  int      `json:"context_window"`
				Temperature    float64  `json:"temperature"`
				Capabilities   []string `json:"capabilities"`
				TimeoutSeconds int      `json:"timeout_seconds"`
				Enabled        bool     `json:"enabled"`
			} `json:"endpoints"`
		}
		if err := json.Unmarshal(encoded, &engine); err != nil || len(engine.Endpoints) == 0 {
			return Provider{}, invalid("AIEngine config is invalid")
		}
		for _, endpoint := range engine.Endpoints {
			if !endpoint.Enabled {
				continue
			}
			config = ProviderConfig{
				ProviderType:   endpoint.ProviderType,
				BaseURL:        endpoint.BaseURL,
				Models:         []ModelConfig{{Name: endpoint.ModelName, ContextWindow: endpoint.ContextWindow, Temperature: endpoint.Temperature, Capabilities: endpoint.Capabilities}},
				TimeoutSeconds: endpoint.TimeoutSeconds,
			}
			break
		}
		if len(config.Models) == 0 {
			return Provider{}, invalid("AIEngine has no enabled endpoint")
		}
	} else if err := json.Unmarshal(encoded, &config); err != nil {
		return Provider{}, invalid("LLMProvider config is invalid")
	}
	if err := validateProviderConfig(config); err != nil {
		return Provider{}, err
	}
	credentialID := ""
	if item.CredentialID != nil {
		credentialID = *item.CredentialID
	}
	return Provider{ResourceID: item.ID, ScopeID: item.ScopeID, Name: item.Name, CredentialID: credentialID, Config: config}, nil
}

func validateProviderConfig(config ProviderConfig) error {
	if !supportedProviderType(config.ProviderType) {
		return invalid("provider_type is not supported")
	}
	parsed, err := url.ParseRequestURI(config.BaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return invalid("base_url must be an absolute HTTP URL")
	}
	if len(config.Models) == 0 || len(config.Models) > 100 {
		return invalid("models must contain 1 to 100 entries")
	}
	names := make([]string, 0, len(config.Models))
	for _, model := range config.Models {
		if strings.TrimSpace(model.Name) == "" || model.ContextWindow <= 0 || model.Temperature < 0 || model.Temperature > 2 {
			return invalid("every model requires a name and positive context_window")
		}
		if slices.Contains(names, model.Name) {
			return invalid("model names must be unique")
		}
		names = append(names, model.Name)
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

func findModel(models []ModelConfig, name string) (ModelConfig, bool) {
	for _, item := range models {
		if item.Name == name {
			return item, true
		}
	}
	return ModelConfig{}, false
}

func allowsScope(ctx context.Context, scopeID string) bool {
	filter, ok := authorization.ScopeFilterFromContext(ctx)
	return !ok || filter.Allows(scopeID)
}

func allowsProviderResource(ctx context.Context, item resource.Resource) bool {
	if filter, ok := authorization.ResourceFilterFromContext(ctx); ok {
		return filter.Allows(item.ScopeID, item.ID)
	}
	return allowsScope(ctx, item.ScopeID)
}
