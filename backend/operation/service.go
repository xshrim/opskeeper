package operation

import (
	"context"
	"opskeeper/backend/authorization"
	"opskeeper/backend/resource"
	"strings"
	"time"
)

type ResourceReader interface {
	Get(context.Context, string) (resource.Resource, error)
}
type Service struct {
	store     Store
	resources ResourceReader
	submitter JobSubmitter
	now       func() time.Time
}

func NewService(store Store, resources ResourceReader) *Service {
	return &Service{store: store, resources: resources, now: time.Now}
}

func NewServiceWithSubmitter(store Store, resources ResourceReader, submitter JobSubmitter) *Service {
	return &Service{store: store, resources: resources, submitter: submitter, now: time.Now}
}
func (s *Service) Request(ctx context.Context, r Request) (Request, error) {
	if err := ValidateRequest(r); err != nil {
		return Request{}, err
	}
	if !allows(ctx, r.ScopeID) {
		return Request{}, authorization.ErrForbidden
	}
	target, err := s.resources.Get(ctx, r.TargetResourceID)
	if err != nil {
		return Request{}, err
	}
	if target.ScopeID != r.ScopeID || target.Status != resource.StatusActive {
		return Request{}, authorization.ErrForbidden
	}
	if filter, ok := authorization.ResourceFilterFromContext(ctx); ok && !filter.Allows(target.ScopeID, target.ID) {
		return Request{}, authorization.ErrForbidden
	}
	hash, err := ParametersHash(r.Parameters)
	if err != nil {
		return Request{}, err
	}
	r.ParametersHash = hash
	// The server constructs the dry-run plan from the canonical parameters.
	// Client-provided text cannot forge what an approver is shown.
	r.DryRun = map[string]any{
		"mode":               "dry_run",
		"operation":          r.OperationName,
		"target_resource_id": r.TargetResourceID,
		"parameters_hash":    hash,
	}
	r.Source = strings.TrimSpace(r.Source)
	if r.Source == "" {
		r.Source = "user"
	}
	if r.Source != "user" && r.Source != "skill" && r.Source != "mcp" {
		return Request{}, ErrConflict
	}
	requiresApproval, expiry := RequiresApproval(r.RiskLevel), 30*time.Minute
	if policies, ok := s.store.(PolicyStore); ok {
		if policy, err := policies.MatchPolicy(ctx, r.ScopeID, target.Kind, r.OperationName); err == nil {
			if riskRank(r.RiskLevel) < riskRank(policy.MinimumRisk) {
				r.RiskLevel = policy.MinimumRisk
			}
			requiresApproval = policy.ApprovalRequired || RequiresApproval(r.RiskLevel)
			if policy.ExpiresAfterSeconds > 0 {
				expiry = time.Duration(policy.ExpiresAfterSeconds) * time.Second
			}
		} else if err != nil && err != ErrNotFound {
			return Request{}, err
		}
	}
	if requiresApproval {
		r.Status = Pending
		expires := s.now().Add(expiry)
		r.ExpiresAt = &expires
	} else {
		r.Status = Approved
	}
	return s.store.Create(ctx, r)
}

func (s *Service) CreatePolicy(ctx context.Context, item Policy) (Policy, error) {
	if !allows(ctx, item.ScopeID) || !IsRisk(item.MinimumRisk) || strings.TrimSpace(item.Name) == "" {
		return Policy{}, authorization.ErrForbidden
	}
	if item.ExpiresAfterSeconds == 0 {
		item.ExpiresAfterSeconds = 1800
	}
	if item.ApproverPermission == "" {
		item.ApproverPermission = string(authorization.OperationApprove)
	}
	policies, ok := s.store.(PolicyStore)
	if !ok {
		return Policy{}, ErrNotFound
	}
	return policies.CreatePolicy(ctx, item)
}
func (s *Service) ListPolicies(ctx context.Context, scopeID string) ([]Policy, error) {
	if !allows(ctx, scopeID) {
		return nil, authorization.ErrForbidden
	}
	policies, ok := s.store.(PolicyStore)
	if !ok {
		return nil, ErrNotFound
	}
	return policies.ListPolicies(ctx, scopeID)
}
func (s *Service) Approve(ctx context.Context, a Approval) (Request, error) {
	r, err := s.store.Get(ctx, a.OperationRequestID)
	if err != nil {
		return Request{}, err
	}
	if !allows(ctx, r.ScopeID) {
		return Request{}, authorization.ErrForbidden
	}
	return s.store.Approve(ctx, a)
}
func (s *Service) Start(ctx context.Context, id, key string) (string, error) {
	r, err := s.store.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if !allows(ctx, r.ScopeID) {
		return "", authorization.ErrForbidden
	}
	target, err := s.resources.Get(ctx, r.TargetResourceID)
	if err != nil {
		return "", err
	}
	if target.ScopeID != r.ScopeID || !allowsResource(ctx, target.ScopeID, target.ID) {
		return "", authorization.ErrForbidden
	}
	executionID, err := s.store.StartExecution(ctx, id, key, s.now())
	if err != nil {
		return "", err
	}
	if s.submitter == nil {
		return executionID, nil
	}
	execution, err := s.store.GetExecution(ctx, executionID)
	if err != nil {
		return "", err
	}
	if execution.Status != "queued" {
		return executionID, nil
	}
	jobName, err := s.submitter.Submit(ctx, r, executionID)
	if err != nil {
		if status, ok := s.store.(interface {
			FailExecution(context.Context, string, string) error
		}); ok {
			_ = status.FailExecution(context.Background(), executionID, err.Error())
		}
		return "", err
	}
	if status, ok := s.store.(interface {
		MarkExecutionRunning(context.Context, string, string) error
	}); ok {
		if err := status.MarkExecutionRunning(ctx, executionID, jobName); err != nil {
			if fail, ok := s.store.(interface {
				FailExecution(context.Context, string, string) error
			}); ok {
				_ = fail.FailExecution(context.Background(), executionID, err.Error())
			}
			return "", err
		}
	}
	return executionID, nil
}

func (s *Service) Get(ctx context.Context, id string) (Request, error) {
	r, err := s.store.Get(ctx, id)
	if err != nil || !allows(ctx, r.ScopeID) {
		if err == nil {
			return Request{}, authorization.ErrForbidden
		}
		return Request{}, err
	}
	if filter, ok := authorization.ResourceFilterFromContext(ctx); ok && !filter.Allows(r.ScopeID, r.TargetResourceID) {
		return Request{}, authorization.ErrForbidden
	}
	return r, nil
}

func (s *Service) List(ctx context.Context, scopeID string, limit int) ([]Request, error) {
	if !allows(ctx, scopeID) {
		return nil, authorization.ErrForbidden
	}
	return s.store.List(ctx, scopeID, limit)
}

func (s *Service) GetExecution(ctx context.Context, executionID string) (Execution, error) {
	item, err := s.store.GetExecution(ctx, executionID)
	if err != nil {
		return Execution{}, err
	}
	r, err := s.store.Get(ctx, item.OperationRequestID)
	if err != nil || !allows(ctx, r.ScopeID) {
		if err == nil {
			return Execution{}, authorization.ErrForbidden
		}
		return Execution{}, err
	}
	return item, nil
}
func allows(ctx context.Context, scope string) bool {
	f, ok := authorization.ScopeFilterFromContext(ctx)
	return !ok || f.Allows(scope)
}

func allowsResource(ctx context.Context, scope, resourceID string) bool {
	if filter, ok := authorization.ResourceFilterFromContext(ctx); ok {
		return filter.Allows(scope, resourceID)
	}
	return allows(ctx, scope)
}

func riskRank(value string) int {
	switch value {
	case RiskReadOnly:
		return 0
	case RiskLow:
		return 1
	case RiskMedium:
		return 2
	case RiskHigh:
		return 3
	default:
		return -1
	}
}
