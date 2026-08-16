// Package operation owns the approval and execution state machine for the
// small, explicitly supported set of state-changing operations.
package operation

import "time"

const (
	RiskReadOnly = "read_only"
	RiskLow      = "low"
	RiskMedium   = "medium"
	RiskHigh     = "high"

	Pending   = "pending"
	Approved  = "approved"
	Rejected  = "rejected"
	Expired   = "expired"
	Executing = "executing"
	Succeeded = "succeeded"
	Failed    = "failed"
)

type Request struct {
	ID               string         `json:"id"`
	ScopeID          string         `json:"scope_id"`
	TargetResourceID string         `json:"target_resource_id"`
	RequestedBy      string         `json:"requested_by"`
	Source           string         `json:"source"`
	OperationName    string         `json:"operation_name"`
	RiskLevel        string         `json:"risk_level"`
	Parameters       map[string]any `json:"parameters"`
	ParametersHash   string         `json:"parameters_hash"`
	ImpactSummary    string         `json:"impact_summary"`
	RollbackSummary  string         `json:"rollback_summary"`
	DryRun           map[string]any `json:"dry_run"`
	IdempotencyKey   string         `json:"idempotency_key"`
	Status           string         `json:"status"`
	ExpiresAt        *time.Time     `json:"expires_at,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type Approval struct {
	ID                 string    `json:"id"`
	OperationRequestID string    `json:"operation_request_id"`
	ApproverUserID     string    `json:"approver_user_id"`
	Decision           string    `json:"decision"`
	ParametersHash     string    `json:"parameters_hash"`
	Comment            string    `json:"comment"`
	CreatedAt          time.Time `json:"created_at"`
}

type Execution struct {
	ID                 string         `json:"id"`
	OperationRequestID string         `json:"operation_request_id"`
	Executor           string         `json:"executor"`
	IdempotencyKey     string         `json:"idempotency_key"`
	Status             string         `json:"status"`
	Result             map[string]any `json:"result"`
	ErrorMessage       string         `json:"error_message,omitempty"`
	StartedAt          *time.Time     `json:"started_at,omitempty"`
	CompletedAt        *time.Time     `json:"completed_at,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type Policy struct {
	ID                  string    `json:"id"`
	ScopeID             string    `json:"scope_id"`
	Name                string    `json:"name"`
	TargetKinds         []string  `json:"target_kinds"`
	OperationNames      []string  `json:"operation_names"`
	MinimumRisk         string    `json:"minimum_risk"`
	ApprovalRequired    bool      `json:"approval_required"`
	ApproverPermission  string    `json:"approver_permission"`
	ExpiresAfterSeconds int       `json:"expires_after_seconds"`
	Status              string    `json:"status"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func RequiresApproval(risk string) bool { return risk == RiskMedium || risk == RiskHigh }
func IsRisk(value string) bool {
	return value == RiskReadOnly || value == RiskLow || value == RiskMedium || value == RiskHigh
}
