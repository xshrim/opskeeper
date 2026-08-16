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
	ID, ScopeID, Name, Cron, Timezone, Status string
	TargetResourceIDs                         []string
	TargetLabels                              map[string]string
	SkillResourceIDs                          []string
	Timeout                                   time.Duration
	Retries, MaxConcurrent                    int
	MaxToolCalls                              int
	MaxTokens                                 int64
	Maintenance                               []MaintenanceWindow
}

// MaintenanceWindow is deliberately a policy fact, not a cron expression:
// Start and End are evaluated in the policy timezone when scheduling.
type MaintenanceWindow struct {
	Start, End time.Time
}

type RuleResult struct {
	TargetResourceID, Rule, Severity, Message string
	Weight                                    int
}

type Finding struct {
	ID, PolicyID, TargetResourceID, Rule, IdentityKey, Fingerprint string
	Severity, Message, Status                                      string
	FirstObservedAt, LastObservedAt                                time.Time
	ResolvedAt                                                     *time.Time
}

type HealthSnapshot struct {
	PolicyID, TargetResourceID string
	Score                      int
	CollectedAt                time.Time
	Reasons                    []RuleResult
}

type Run struct {
	ID, PolicyID, ScopeID, Trigger, Status, LLMStatus string
	WindowStart, WindowEnd                            time.Time
	Score                                             *int
	DeterministicCompleted                            bool
	ErrorCode, ErrorMessage                           string
	StartedAt, CompletedAt                            *time.Time
}

type NotificationChannel struct {
	ID, ScopeID, Name, Kind, WebhookURL, Status string
	CredentialID                                *string
	RateLimitPerMinute                          int
}

type Delivery struct {
	ID, ChannelID, FindingID, RunID, IdempotencyKey, Status string
	Attempt, ResponseStatus                                 int
	ResponseBody, ErrorMessage                              string
}
