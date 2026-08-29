// Package aiengine provides the shared execution contract for AI-powered
// features. Provider and resource resolution stays outside this package.
package aiengine

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Purpose mirrors llm.Purpose without coupling the execution package to the
// provider implementation.
type Purpose string

const (
	PurposeDefault    Purpose = "default"
	PurposeDiagnosis  Purpose = "diagnosis"
	PurposeInspection Purpose = "inspection"
	PurposeWorkflow   Purpose = "workflow"
)

type Profile string

const (
	ProfileInteractive Profile = "interactive"
	ProfileDiagnosis   Profile = "diagnosis"
	ProfileSkill       Profile = "skill"
	ProfileInspection  Profile = "inspection"
	ProfileWorkflow    Profile = "workflow"
)

type Status string

const (
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

var (
	ErrRunnerUnavailable = errors.New("AIEngine runner is unavailable")
	ErrInvalidRequest    = errors.New("AIEngine request is invalid")
	ErrAlreadyRunning    = errors.New("AIEngine execution is already running")
)

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }
func (e *ValidationError) Unwrap() error { return ErrInvalidRequest }

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ContextRequest struct {
	ResourceIDs      []string `json:"resource_ids,omitempty"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids,omitempty"`
}

type Requirements struct {
	Capabilities  []string `json:"capabilities,omitempty"`
	MinContext    int      `json:"min_context,omitempty"`
	StructuredOut bool     `json:"structured_output,omitempty"`
	Streaming     bool     `json:"streaming,omitempty"`
}

type Budget struct {
	MaxIterations  int           `json:"max_iterations,omitempty"`
	MaxToolCalls   int           `json:"max_tool_calls,omitempty"`
	MaxTokens      int64         `json:"max_tokens,omitempty"`
	MaxOutputBytes int           `json:"max_output_bytes,omitempty"`
	Timeout        time.Duration `json:"timeout,omitempty"`
}

func DefaultBudget() Budget {
	return Budget{MaxIterations: 12, MaxToolCalls: 12, MaxTokens: 20000, MaxOutputBytes: 64 << 10, Timeout: 2 * time.Minute}
}

type Request struct {
	ExecutionID          string         `json:"execution_id,omitempty"`
	ActorID              string         `json:"actor_id,omitempty"`
	ScopeID              string         `json:"scope_id"`
	Purpose              Purpose        `json:"purpose,omitempty"`
	AIProviderResourceID string         `json:"ai_provider_resource_id,omitempty"`
	ModelName            string         `json:"model_name,omitempty"`
	Profile              Profile        `json:"profile"`
	Task                 string         `json:"task,omitempty"`
	Messages             []Message      `json:"messages,omitempty"`
	Input                map[string]any `json:"input,omitempty"`
	Context              ContextRequest `json:"context,omitempty"`
	SkillResourceID      string         `json:"skill_resource_id,omitempty"`
	SkillVersionID       string         `json:"skill_version_id,omitempty"`
	AgentProfileID       string         `json:"agent_profile_id,omitempty"`
	WorkflowID           string         `json:"workflow_id,omitempty"`
	Requirements         Requirements   `json:"requirements,omitempty"`
	Budget               Budget         `json:"budget,omitempty"`
	Stream               bool           `json:"stream,omitempty"`

	EventSink       EventSink        `json:"-"`
	ResolvedContext *ResolvedContext `json:"-"`
	ToolGateway     *PolicyGateway   `json:"-"`
}

func (r *Request) Normalize() error {
	if r == nil {
		return &ValidationError{Message: "request is required"}
	}
	r.ExecutionID = strings.TrimSpace(r.ExecutionID)
	r.ActorID = strings.TrimSpace(r.ActorID)
	r.ScopeID = strings.TrimSpace(r.ScopeID)
	r.AIProviderResourceID = strings.TrimSpace(r.AIProviderResourceID)
	r.ModelName = strings.TrimSpace(r.ModelName)
	r.Task = strings.TrimSpace(r.Task)
	r.SkillResourceID = strings.TrimSpace(r.SkillResourceID)
	r.SkillVersionID = strings.TrimSpace(r.SkillVersionID)
	r.AgentProfileID = strings.TrimSpace(r.AgentProfileID)
	r.WorkflowID = strings.TrimSpace(r.WorkflowID)
	if r.ScopeID == "" {
		return &ValidationError{Message: "scope_id is required"}
	}
	if r.Profile == "" {
		r.Profile = ProfileInteractive
	}
	if r.Purpose == "" {
		r.Purpose = PurposeDefault
	}
	switch r.Purpose {
	case PurposeDefault, PurposeDiagnosis, PurposeInspection, PurposeWorkflow:
	default:
		return &ValidationError{Message: "purpose is unsupported"}
	}
	switch r.Profile {
	case ProfileInteractive, ProfileDiagnosis, ProfileSkill, ProfileInspection, ProfileWorkflow:
	default:
		return &ValidationError{Message: "profile is unsupported"}
	}
	if r.Task == "" && len(r.Messages) == 0 {
		return &ValidationError{Message: "task or messages is required"}
	}
	if r.Budget.MaxIterations <= 0 {
		r.Budget.MaxIterations = DefaultBudget().MaxIterations
	}
	if r.Budget.MaxToolCalls <= 0 {
		r.Budget.MaxToolCalls = DefaultBudget().MaxToolCalls
	}
	if r.Budget.MaxTokens <= 0 {
		r.Budget.MaxTokens = DefaultBudget().MaxTokens
	}
	if r.Budget.MaxOutputBytes <= 0 {
		r.Budget.MaxOutputBytes = DefaultBudget().MaxOutputBytes
	}
	if r.Budget.Timeout <= 0 {
		r.Budget.Timeout = DefaultBudget().Timeout
	}
	if r.Budget.Timeout > 30*time.Minute {
		return &ValidationError{Message: "timeout must not exceed 30 minutes"}
	}
	return nil
}

type Event struct {
	Sequence    int64          `json:"sequence"`
	ExecutionID string         `json:"execution_id"`
	Type        string         `json:"type"`
	Status      Status         `json:"status,omitempty"`
	Timestamp   time.Time      `json:"timestamp"`
	Payload     map[string]any `json:"payload,omitempty"`
}

type EventSink func(Event) error

type Result struct {
	ExecutionID      string `json:"execution_id"`
	Status           Status `json:"status"`
	Output           string `json:"output,omitempty"`
	PromptTokens     int64  `json:"prompt_tokens,omitempty"`
	CompletionTokens int64  `json:"completion_tokens,omitempty"`
	TotalTokens      int64  `json:"total_tokens,omitempty"`
	ToolCallCount    int    `json:"tool_call_count,omitempty"`
	ErrorCode        string `json:"error_code,omitempty"`
	ErrorMessage     string `json:"error_message,omitempty"`
}

type Runner interface {
	Run(context.Context, Request) (Result, error)
}

type StreamingRunner interface {
	RunStream(context.Context, Request, EventSink) (Result, error)
}

type Engine interface {
	Name() string
	Execute(context.Context, Request) (Result, error)
	Stream(context.Context, Request) (<-chan Event, error)
	Cancel(context.Context, string) error
}
