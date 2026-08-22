package llm

import "time"

const ProviderKind = "LLMProvider"
const AIEngineKind = "AIEngine"

type ModelConfig struct {
	Name                  string   `json:"name"`
	ContextWindow         int      `json:"context_window"`
	Temperature           float64  `json:"temperature"`
	InputPricePerMillion  float64  `json:"input_price_per_million,omitempty"`
	OutputPricePerMillion float64  `json:"output_price_per_million,omitempty"`
	Capabilities          []string `json:"capabilities,omitempty"`
}

type ProviderConfig struct {
	ProviderType   string        `json:"provider_type"`
	BaseURL        string        `json:"base_url"`
	Models         []ModelConfig `json:"models"`
	TimeoutSeconds int           `json:"timeout_seconds,omitempty"`
}

type Provider struct {
	ResourceID   string         `json:"resource_id"`
	ScopeID      string         `json:"scope_id"`
	Name         string         `json:"name"`
	CredentialID string         `json:"-"`
	Config       ProviderConfig `json:"config"`
}

type Default struct {
	ScopeID            string    `json:"scope_id"`
	ProviderResourceID string    `json:"provider_resource_id"`
	ModelName          string    `json:"model_name"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type ResolvedModel struct {
	Provider         Provider    `json:"provider"`
	Model            ModelConfig `json:"model"`
	DefinedAtScopeID string      `json:"defined_at_scope_id"`
	APIKey           string      `json:"-"`
}

type ConnectionResult struct {
	ProviderResourceID string `json:"provider_resource_id"`
	ModelName          string `json:"model_name"`
	Status             string `json:"status"`
	LatencyMS          int64  `json:"latency_ms"`
	Message            string `json:"message"`
}

type DraftConnection struct {
	ScopeID       string
	ProviderType  string
	BaseURL       string
	ModelName     string
	APIKey        string
	ContextWindow int
	Temperature   float64
	Capabilities  []string
}
