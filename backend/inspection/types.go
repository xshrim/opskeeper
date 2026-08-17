// Package inspection owns scheduled, read-only health inspection policies and
// their durable execution lifecycle.
package inspection

import "time"

const (
	PolicyActive   = "active"
	PolicyDisabled = "disabled"

	FindingOpen     = "open"
	FindingResolved = "resolved"

	RunQueued    = "queued"
	RunRunning   = "running"
	RunSucceeded = "succeeded"
	RunFailed    = "failed"
	RunSkipped   = "skipped"
)

type Policy struct {
	ID                string              `json:"id"`
	ScopeID           string              `json:"scope_id"`
	Name              string              `json:"name"`
	Cron              string              `json:"cron"`
	Timezone          string              `json:"timezone"`
	Status            string              `json:"status"`
	TargetResourceIDs []string            `json:"target_resource_ids"`
	TargetLabels      map[string]string   `json:"target_labels"`
	SkillResourceIDs  []string            `json:"skill_resource_ids"`
	Timeout           time.Duration       `json:"timeout"`
	TimeoutSeconds    int                 `json:"timeout_seconds"`
	Retries           int                 `json:"retries"`
	MaxConcurrent     int                 `json:"max_concurrent"`
	MaxToolCalls      int                 `json:"max_tool_calls"`
	MaxTokens         int64               `json:"max_tokens"`
	Maintenance       []MaintenanceWindow `json:"maintenance"`
}

// MaintenanceWindow is deliberately a policy fact, not a cron expression:
// Start and End are evaluated in the policy timezone when scheduling.
type MaintenanceWindow struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type RuleResult struct {
	TargetResourceID string `json:"target_resource_id"`
	Rule             string `json:"rule"`
	Severity         string `json:"severity"`
	Message          string `json:"message"`
	Weight           int    `json:"weight"`
}

type Finding struct {
	ID               string     `json:"id"`
	PolicyID         string     `json:"policy_id"`
	TargetResourceID string     `json:"target_resource_id"`
	Rule             string     `json:"rule"`
	IdentityKey      string     `json:"identity_key"`
	Fingerprint      string     `json:"fingerprint"`
	Severity         string     `json:"severity"`
	Message          string     `json:"message"`
	Status           string     `json:"status"`
	FirstObservedAt  time.Time  `json:"first_observed_at"`
	LastObservedAt   time.Time  `json:"last_observed_at"`
	ResolvedAt       *time.Time `json:"resolved_at,omitempty"`
}

type HealthSnapshot struct {
	PolicyID         string       `json:"policy_id"`
	TargetResourceID string       `json:"target_resource_id"`
	Score            int          `json:"score"`
	CollectedAt      time.Time    `json:"collected_at"`
	Reasons          []RuleResult `json:"reasons"`
}

type Run struct {
	ID                     string     `json:"id"`
	PolicyID               string     `json:"policy_id"`
	ScopeID                string     `json:"scope_id"`
	Trigger                string     `json:"trigger"`
	Status                 string     `json:"status"`
	LLMStatus              string     `json:"llm_status"`
	WindowStart            time.Time  `json:"window_start"`
	WindowEnd              time.Time  `json:"window_end"`
	Score                  *int       `json:"score,omitempty"`
	DeterministicCompleted bool       `json:"deterministic_completed"`
	ErrorCode              string     `json:"error_code,omitempty"`
	ErrorMessage           string     `json:"error_message,omitempty"`
	StartedAt              *time.Time `json:"started_at,omitempty"`
	CompletedAt            *time.Time `json:"completed_at,omitempty"`
}

type NotificationChannel struct {
	ID                 string  `json:"id"`
	ScopeID            string  `json:"scope_id"`
	Name               string  `json:"name"`
	Kind               string  `json:"kind"`
	WebhookURL         string  `json:"webhook_url"`
	Status             string  `json:"status"`
	CredentialID       *string `json:"credential_id,omitempty"`
	RateLimitPerMinute int     `json:"rate_limit_per_minute"`
}

type Delivery struct {
	ID, ChannelID, FindingID, RunID, IdempotencyKey, Status string
	Attempt, ResponseStatus                                 int
	ResponseBody, ErrorMessage                              string
}
