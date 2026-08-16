package operation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

var ErrApprovalRequired = errors.New("operation approval is required")
var ErrApprovalInvalid = errors.New("operation approval is invalid")

func ParametersHash(value map[string]any) (string, error) {
	normalized := normalize(value)
	raw, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func normalize(value any) any {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(v))
		for _, key := range keys {
			out[key] = normalize(v[key])
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = normalize(v[i])
		}
		return out
	default:
		return value
	}
}
func CanExecute(request Request, approval Approval, now time.Time) error {
	if request.Status != Approved {
		return ErrApprovalRequired
	}
	if request.ExpiresAt != nil && !now.Before(*request.ExpiresAt) {
		return ErrApprovalInvalid
	}
	if approval.Decision != "approved" || approval.ParametersHash != request.ParametersHash {
		return ErrApprovalInvalid
	}
	if approval.ApproverUserID == request.RequestedBy {
		return ErrApprovalInvalid
	}
	return nil
}
func ValidateRequest(input Request) error {
	if strings.TrimSpace(input.ScopeID) == "" || strings.TrimSpace(input.TargetResourceID) == "" || strings.TrimSpace(input.RequestedBy) == "" || strings.TrimSpace(input.OperationName) == "" {
		return errors.New("scope, target, requester and operation are required")
	}
	if !IsRisk(input.RiskLevel) {
		return errors.New("invalid operation risk")
	}
	if input.RiskLevel == RiskHigh && strings.Contains(strings.ToLower(input.OperationName), "delete") {
		return errors.New("high-risk delete operations are disabled")
	}
	if strings.Contains(strings.ToLower(input.OperationName), "sql") {
		return errors.New("write SQL operations are disabled")
	}
	if input.OperationName != RestartWorkload && input.OperationName != ScaleWorkload {
		return errors.New("operation is not in the approved catalog")
	}
	return nil
}
