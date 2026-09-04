// Package aiengine provides the shared execution contract for AI-powered
// features. Provider and resource resolution stays outside this package.
package aiengine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"google.golang.org/adk/v2/model"
)

// Purpose mirrors llm.Purpose without coupling the execution package to the
// provider implementation.
type Purpose string

const (
	PurposeGeneral    Purpose = "general"
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

// ToolDeclaration is the provider-neutral tool contract supplied by an
// optional execution plan. AIEngine owns invocation, policy and audit; plan
// sources only declare what may be exposed to the model.
type ToolDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// ExecutionPlan is the complete, immutable instruction contract for one
// execution. Skill and future plan sources may produce it, but never execute
// it. This keeps the Agent Loop and all model/tool policy in AIEngine.
type ExecutionPlan struct {
	SourceResourceID string            `json:"source_resource_id,omitempty"`
	SourceVersionID  string            `json:"source_version_id,omitempty"`
	Name             string            `json:"name"`
	Description      string            `json:"description,omitempty"`
	Instruction      string            `json:"instruction"`
	InputSchema      json.RawMessage   `json:"input_schema,omitempty"`
	OutputSchema     json.RawMessage   `json:"output_schema,omitempty"`
	Tools            []ToolDeclaration `json:"tools,omitempty"`
	Capabilities     []string          `json:"capabilities,omitempty"`
	AllowedTools     []string          `json:"allowed_tools,omitempty"`
	TargetKinds      []string          `json:"target_kinds,omitempty"`
}

// PlanResolver resolves an optional plan source. It is deliberately narrower
// than a runner: resolving a plan has no model calls or side effects.
type PlanResolver interface {
	ResolvePlan(context.Context, string, string, string) (ExecutionPlan, error)
}

// AgentProfile is a versioned, resource-backed expert prompt contract. It
// narrows the tools and model capabilities an execution may use; it never
// carries provider credentials or an upstream URL.
type AgentProfile struct {
	ResourceID   string          `json:"resource_id"`
	ScopeID      string          `json:"scope_id"`
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	Version      int             `json:"version"`
	Instruction  string          `json:"instruction"`
	Capabilities []string        `json:"capabilities,omitempty"`
	AllowedTools []string        `json:"allowed_tools,omitempty"`
	TargetKinds  []string        `json:"target_kinds,omitempty"`
	InputSchema  json.RawMessage `json:"input_schema,omitempty"`
	OutputSchema json.RawMessage `json:"output_schema,omitempty"`
	Enabled      bool            `json:"enabled"`
}

// AgentProfileResolver resolves and authorizes a profile for an execution
// scope. Implementations should return only active, permitted profiles.
type AgentProfileResolver interface {
	Resolve(context.Context, string, string) (AgentProfile, error)
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
	// MaxOutputTokens caps one provider response independently from the
	// cumulative execution token budget. Zero lets the selected model decide.
	MaxOutputTokens int `json:"max_output_tokens,omitempty"`
}

func DefaultBudget() Budget {
	// Long-running diagnosis may require many model/tool turns. Keep a finite
	// upper bound, but leave enough time for the execution loop to react to
	// real environment feedback instead of failing after a couple of minutes.
	return Budget{MaxIterations: 100, MaxToolCalls: 100, MaxTokens: 1000000, MaxOutputTokens: 128000, MaxOutputBytes: 64 << 10, Timeout: 30 * time.Minute}
}

type Request struct {
	ExecutionID          string          `json:"execution_id,omitempty"`
	ActorID              string          `json:"actor_id,omitempty"`
	ScopeID              string          `json:"scope_id"`
	Purpose              Purpose         `json:"purpose,omitempty"`
	AIProviderResourceID string          `json:"ai_provider_resource_id,omitempty"`
	ModelName            string          `json:"model_name,omitempty"`
	Profile              Profile         `json:"profile"`
	Task                 string          `json:"task,omitempty"`
	Messages             []Message       `json:"messages,omitempty"`
	Input                map[string]any  `json:"input,omitempty"`
	Context              ContextRequest  `json:"context,omitempty"`
	SkillResourceID      string          `json:"skill_resource_id,omitempty"`
	SkillVersionID       string          `json:"skill_version_id,omitempty"`
	AgentProfileID       string          `json:"agent_profile_id,omitempty"`
	WorkflowID           string          `json:"workflow_id,omitempty"`
	Instruction          string          `json:"instruction,omitempty"`
	InputSchema          json.RawMessage `json:"input_schema,omitempty"`
	OutputSchema         json.RawMessage `json:"output_schema,omitempty"`
	AllowedTools         []string        `json:"allowed_tools,omitempty"`
	RestrictTools        bool            `json:"restrict_tools,omitempty"`
	Requirements         Requirements    `json:"requirements,omitempty"`
	Budget               Budget          `json:"budget,omitempty"`
	Stream               bool            `json:"stream,omitempty"`

	EventSink            EventSink             `json:"-"`
	ResolvedContext      *ResolvedContext      `json:"-"`
	ToolGateway          *PolicyGateway        `json:"-"`
	ResolvedAgentProfile *AgentProfile         `json:"-"`
	ObservationSink      func(ToolObservation) `json:"-"`
}

// ToolObservation is emitted after a generic context tool returns. Business
// adapters may turn it into domain evidence without coupling AIEngine to a
// Connector or Skill package.
type ToolObservation struct {
	ToolName   string     `json:"tool_name"`
	ResourceID string     `json:"resource_id"`
	Iteration  int        `json:"iteration,omitempty"`
	CallID     string     `json:"call_id,omitempty"`
	Result     ToolResult `json:"result"`
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
		r.Purpose = PurposeGeneral
	}
	switch r.Purpose {
	case PurposeGeneral, PurposeDiagnosis, PurposeInspection, PurposeWorkflow:
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
	if r.Budget.MaxOutputTokens <= 0 {
		r.Budget.MaxOutputTokens = DefaultBudget().MaxOutputTokens
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

// EventContextCompacted marks a context-window compaction boundary. It is a
// lifecycle event, not a tool invocation, and does not affect tool budgets.
const EventContextCompacted = "context.compacted"

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

// ModelBuildResult is the provider-independent model handle consumed by the
// generic AgentRunner. Provider resolution and credential loading remain in
// the llm package; the execution runtime only receives a model client and
// the capabilities of the selected model.
type ModelBuildResult struct {
	Client              model.LLM
	ProviderResourceID  string
	ModelName           string
	Capabilities        []string
	ContextWindowTokens int
	MaxOutputTokens     int
	// Temperature is the selected provider model's generation parameter. It is
	// applied to every model request by the runner instead of being left only
	// in the resource configuration.
	Temperature float64
}

// ModelBuilder resolves an AIProvider and model for one execution. Keeping
// this as a small function avoids coupling the execution package to the
// provider/resource persistence implementation.
type ModelBuilder func(context.Context, string, string, string, Purpose) (ModelBuildResult, error)
