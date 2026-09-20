package skill

import (
	"encoding/json"
	"time"
)

const Kind = "Skill"

type Skill struct {
	ID         string    `json:"id"`
	ScopeID    string    `json:"scope_id"`
	Name       string    `json:"name"`
	Identifier string    `json:"identifier"`
	Category   string    `json:"category"`
	Tags       []string  `json:"tags"`
	Maintainer string    `json:"maintainer"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

const (
	CategoryDiagnosis    = "diagnosis"
	CategoryMonitoring   = "monitoring"
	CategoryOptimization = "optimization"
	CategoryMaintenance  = "maintenance"

	BodyFormatJSON     = "json"
	BodyFormatYAML     = "yaml"
	BodyFormatMarkdown = "markdown"

	DocumentAPIVersion = "opskeeper.skill/v1"
	DocumentKind       = "Skill"
)

// Metadata and Document describe the portable Skill contract. Scope remains
// the owning Resource's scope_id; it is intentionally absent here.
type Metadata struct {
	Name       string   `json:"name"`
	Identifier string   `json:"identifier"`
	Version    string   `json:"version"`
	Category   string   `json:"category"`
	Tags       []string `json:"tags,omitempty"`
	Maintainer string   `json:"maintainer"`
}

type Document struct {
	APIVersion string         `json:"api_version"`
	Kind       string         `json:"kind"`
	Metadata   Metadata       `json:"metadata"`
	Spec       map[string]any `json:"spec"`
}

type ParsedDocument struct {
	Format        string
	Raw           string
	Document      Document
	Normalized    json.RawMessage
	ContentSHA256 string
}

type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type Manifest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Instruction string `json:"instruction"`
}

type Version struct {
	ID           string          `json:"id"`
	SkillID      string          `json:"skill_id"`
	Version      int             `json:"version"`
	Manifest     Manifest        `json:"manifest"`
	InputSchema  json.RawMessage `json:"input_schema"`
	OutputSchema json.RawMessage `json:"output_schema"`
	Tools        []ToolSpec      `json:"tools"`
	RiskLevel    string          `json:"risk_level"`
	Status       string          `json:"status"`
	CreatedBy    *string         `json:"created_by,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	PublishedAt  *time.Time      `json:"published_at,omitempty"`
}

type CreateVersionInput struct {
	SkillID      string
	Manifest     Manifest
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage
	Tools        []ToolSpec
	RiskLevel    string
	CreatedBy    string
}

type Default struct {
	ScopeID        string    `json:"scope_id"`
	SkillID        string    `json:"skill_id"`
	SkillVersionID string    `json:"skill_version_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
