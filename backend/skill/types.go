package skill

import (
	"encoding/json"
	"time"
)

const Kind = "Skill"

type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type Manifest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Instruction string   `json:"instruction"`
	TargetKinds []string `json:"target_kinds"`
}

type Version struct {
	ID              string          `json:"id"`
	SkillResourceID string          `json:"skill_resource_id"`
	Version         int             `json:"version"`
	Manifest        Manifest        `json:"manifest"`
	InputSchema     json.RawMessage `json:"input_schema"`
	OutputSchema    json.RawMessage `json:"output_schema"`
	Tools           []ToolSpec      `json:"tools"`
	RiskLevel       string          `json:"risk_level"`
	Status          string          `json:"status"`
	CreatedBy       *string         `json:"created_by,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	PublishedAt     *time.Time      `json:"published_at,omitempty"`
}

type CreateVersionInput struct {
	SkillResourceID string
	Manifest        Manifest
	InputSchema     json.RawMessage
	OutputSchema    json.RawMessage
	Tools           []ToolSpec
	RiskLevel       string
	CreatedBy       string
}

type Default struct {
	ScopeID         string    `json:"scope_id"`
	SkillResourceID string    `json:"skill_resource_id"`
	SkillVersionID  string    `json:"skill_version_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
