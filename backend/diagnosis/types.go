package diagnosis

import (
	"encoding/json"
	"time"
)

type Status string

const (
	StatusQueued     Status = "queued"
	StatusPlanning   Status = "planning"
	StatusCollecting Status = "collecting"
	StatusAnalyzing  Status = "analyzing"
	StatusSucceeded  Status = "succeeded"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

type Session struct {
	ID                 string     `json:"id"`
	ScopeID            string     `json:"scope_id"`
	ProviderResourceID string     `json:"ai_provider_resource_id,omitempty"`
	ModelName          string     `json:"model_name,omitempty"`
	Title              string     `json:"title"`
	ErrorCode          string     `json:"error_code"`
	ErrorMessage       string     `json:"error_message"`
	ActorUserID        *string    `json:"actor_user_id,omitempty"`
	Status             Status     `json:"status"`
	StartedAt          time.Time  `json:"started_at"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type Target struct {
	SessionID  string    `json:"session_id"`
	ResourceID string    `json:"resource_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type Message struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type Plan struct {
	ID        string     `json:"id"`
	SessionID string     `json:"session_id"`
	Summary   string     `json:"summary"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Steps     []PlanStep `json:"steps,omitempty"`
}

type PlanStep struct {
	ID        string    `json:"id"`
	PlanID    string    `json:"plan_id"`
	Phase     string    `json:"phase"`
	Status    string    `json:"status"`
	Title     string    `json:"title"`
	Detail    string    `json:"detail"`
	Sequence  int       `json:"sequence"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Event struct {
	ID        int64           `json:"id"`
	SessionID string          `json:"session_id"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

type Evidence struct {
	ID               string          `json:"id"`
	SessionID        string          `json:"session_id"`
	Capability       string          `json:"capability"`
	ContentHash      string          `json:"content_hash"`
	TargetResourceID *string         `json:"target_resource_id,omitempty"`
	SourceResourceID *string         `json:"source_resource_id,omitempty"`
	CollectedAt      time.Time       `json:"collected_at"`
	WindowStart      *time.Time      `json:"window_start,omitempty"`
	WindowEnd        *time.Time      `json:"window_end,omitempty"`
	Summary          json.RawMessage `json:"summary"`
	Content          json.RawMessage `json:"content"`
	Partial          bool            `json:"partial"`
	Untrusted        bool            `json:"untrusted"`
	CreatedAt        time.Time       `json:"created_at"`
}

type Hypothesis struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id"`
	Statement   string    `json:"statement"`
	Status      string    `json:"status"`
	Confidence  float64   `json:"confidence"`
	EvidenceIDs []string  `json:"evidence_ids"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Report struct {
	ID              string          `json:"id"`
	SessionID       string          `json:"session_id"`
	Status          string          `json:"status"`
	Conclusion      string          `json:"conclusion"`
	Recommendations json.RawMessage `json:"recommendations"`
	EvidenceIDs     []string        `json:"evidence_ids"`
	CreatedAt       time.Time       `json:"created_at"`
}

type Snapshot struct {
	Session    Session      `json:"session"`
	Targets    []Target     `json:"targets"`
	Messages   []Message    `json:"messages"`
	Plan       *Plan        `json:"plan,omitempty"`
	Evidence   []Evidence   `json:"evidence"`
	Hypotheses []Hypothesis `json:"hypotheses"`
	Report     *Report      `json:"report,omitempty"`
}

type StartInput struct {
	ScopeID, ActorUserID, Title, Question string
	ProviderResourceID, ModelName         string
	TargetResourceIDs                     []string
}

type AppendMessageInput struct {
	Role, Content string
}

type CreateEventInput struct {
	Type    string
	Payload any
}

type CreateEvidenceInput struct {
	TargetResourceID, SourceResourceID, Capability string
	CollectedAt                                    time.Time
	WindowStart, WindowEnd                         *time.Time
	Summary, Content                               json.RawMessage
	Partial, Untrusted                             bool
}
