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

type Execution struct {
	ID                 string     `json:"id"`
	ScopeID            string     `json:"scope_id"`
	ActorUserID        *string    `json:"actor_user_id,omitempty"`
	TargetResourceID   *string    `json:"target_resource_id,omitempty"`
	SkillResourceID    string     `json:"skill_resource_id"`
	SkillVersionID     string     `json:"skill_version_id"`
	ProviderResourceID string     `json:"provider_resource_id"`
	ModelName          string     `json:"model_name"`
	Status             string     `json:"status"`
	InputDigest        string     `json:"input_digest"`
	OutputPreview      string     `json:"output_preview,omitempty"`
	PromptTokens       int64      `json:"prompt_tokens"`
	CompletionTokens   int64      `json:"completion_tokens"`
	TotalTokens        int64      `json:"total_tokens"`
	ToolCallCount      int        `json:"tool_call_count"`
	ErrorCode          string     `json:"error_code,omitempty"`
	ErrorMessage       string     `json:"error_message,omitempty"`
	StartedAt          time.Time  `json:"started_at"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
}

type StartExecutionInput struct {
	ScopeID, ActorUserID, TargetResourceID string
	SkillResourceID, SkillVersionID        string
	ProviderResourceID, ModelName          string
	InputDigest                            string
}

type FinishExecutionInput struct {
	Status, OutputPreview, ErrorCode, ErrorMessage string
	PromptTokens, CompletionTokens, TotalTokens    int64
	ToolCallCount                                  int
}

type ToolCall struct {
	ID               string     `json:"id"`
	ExecutionID      string     `json:"execution_id"`
	Sequence         int        `json:"sequence"`
	ToolName         string     `json:"tool_name"`
	TargetResourceID *string    `json:"target_resource_id,omitempty"`
	Status           string     `json:"status"`
	InputDigest      string     `json:"input_digest"`
	OutputPreview    string     `json:"output_preview,omitempty"`
	ErrorCode        string     `json:"error_code,omitempty"`
	ErrorMessage     string     `json:"error_message,omitempty"`
	StartedAt        time.Time  `json:"started_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}
