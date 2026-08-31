package llm

import "time"

const AIProviderKind = "AIProvider"

// Purpose identifies the execution scenario whose routed provider is used.
type Purpose string

const (
	PurposeGeneral    Purpose = "general"
	PurposeDiagnosis  Purpose = "diagnosis"
	PurposeInspection Purpose = "inspection"
	PurposeWorkflow   Purpose = "workflow"
)

func (p Purpose) String() string { return string(p) }

type ProviderModel struct {
	Name                string   `json:"name"`
	ContextWindowTokens int      `json:"context_window_tokens"`
	MaxOutputTokens     int      `json:"max_output_tokens,omitempty"`
	Temperature         float64  `json:"temperature,omitempty"`
	TemperatureMutable  bool     `json:"temperature_mutable,omitempty"`
	Capabilities        []string `json:"capabilities"`
	Enabled             bool     `json:"enabled"`
	Priority            int      `json:"priority,omitempty"`
}

type AIProviderConfig struct {
	ProviderType       string          `json:"provider_type"`
	Protocol           string          `json:"protocol,omitempty"`
	BaseURL            string          `json:"base_url"`
	TimeoutSeconds     int             `json:"timeout_seconds,omitempty"`
	MaxConcurrency     int             `json:"max_concurrency,omitempty"`
	RateLimitPerMinute int             `json:"rate_limit_per_minute,omitempty"`
	Enabled            bool            `json:"enabled"`
	DefaultModel       string          `json:"default_model,omitempty"`
	Models             []ProviderModel `json:"models"`
}

type AIProvider struct {
	ResourceID   string           `json:"resource_id"`
	ScopeID      string           `json:"scope_id"`
	Name         string           `json:"name"`
	CredentialID string           `json:"-"`
	Config       AIProviderConfig `json:"config"`
}

type ScopeProviderBinding struct {
	ScopeID    string    `json:"scope_id"`
	ProviderID string    `json:"provider_resource_id"`
	Tag        Purpose   `json:"tag"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ResolvedProvider struct {
	ProviderResourceID string        `json:"provider_resource_id"`
	ProviderName       string        `json:"provider_name"`
	Provider           AIProvider    `json:"provider"`
	Model              ProviderModel `json:"model"`
	DefinedAtScopeID   string        `json:"defined_at_scope_id"`
	SelectionReason    string        `json:"selection_reason,omitempty"`
	APIKey             string        `json:"-"`
}

type ConnectionResult struct {
	ProviderResourceID string `json:"provider_resource_id,omitempty"`
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
