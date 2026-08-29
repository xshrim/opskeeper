package inspection

import (
	"context"
	"strings"
	"time"

	"opskeeper/backend/authorization"
	"opskeeper/backend/resource"
)

type ResourceReader interface {
	Get(context.Context, string) (resource.Resource, error)
}

type Service struct {
	store     policyStore
	resources ResourceReader
}

type policyStore interface {
	CreatePolicy(context.Context, Policy, string) (Policy, error)
	ListPolicies(context.Context, string) ([]Policy, error)
}

func NewService(store policyStore, resources ResourceReader) *Service {
	return &Service{store: store, resources: resources}
}

func (s *Service) CreatePolicy(ctx context.Context, input Policy, actorID string) (Policy, error) {
	policy, err := normalizePolicy(input)
	if err != nil {
		return Policy{}, err
	}
	if !allowsScope(ctx, policy.ScopeID) {
		return Policy{}, authorization.ErrForbidden
	}
	for _, targetID := range policy.TargetResourceIDs {
		if err := s.validateResource(ctx, policy.ScopeID, targetID, ""); err != nil {
			return Policy{}, err
		}
	}
	if policy.AgentProfileResourceID != "" {
		if err := s.validateResource(ctx, policy.ScopeID, policy.AgentProfileResourceID, "AgentProfile"); err != nil {
			return Policy{}, err
		}
	}
	return s.store.CreatePolicy(ctx, policy, strings.TrimSpace(actorID))
}

func (s *Service) ListPolicies(ctx context.Context, scopeID string) ([]Policy, error) {
	scopeID = strings.TrimSpace(scopeID)
	if scopeID == "" || !allowsScope(ctx, scopeID) {
		return nil, authorization.ErrForbidden
	}
	return s.store.ListPolicies(ctx, scopeID)
}

type operationalStore interface {
	policyStore
	ResolveTargets(context.Context, Policy) ([]string, error)
	CreateManualRun(context.Context, Policy, time.Time, []string) (string, error)
	ListRuns(context.Context, string, int) ([]Run, error)
	ListFindings(context.Context, string, int) ([]Finding, error)
	CreateChannel(context.Context, NotificationChannel) (NotificationChannel, error)
	ListChannels(context.Context, string) ([]NotificationChannel, error)
}

func (s *Service) StartManualRun(ctx context.Context, scopeID, policyID string, now time.Time) (string, error) {
	if !allowsScope(ctx, scopeID) {
		return "", authorization.ErrForbidden
	}
	store, ok := s.store.(operationalStore)
	if !ok {
		return "", invalid("inspection run store is unavailable")
	}
	items, err := store.ListPolicies(ctx, scopeID)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if item.ID == strings.TrimSpace(policyID) {
			if item.Status != PolicyActive {
				return "", ErrConflict
			}
			targets, err := store.ResolveTargets(ctx, item)
			if err != nil {
				return "", err
			}
			if len(targets) == 0 {
				return "", ErrConflict
			}
			item.TargetResourceIDs = targets
			return store.CreateManualRun(ctx, item, now, targets)
		}
	}
	return "", ErrNotFound
}

func (s *Service) ListRuns(ctx context.Context, scopeID string, limit int) ([]Run, error) {
	if !allowsScope(ctx, scopeID) {
		return nil, authorization.ErrForbidden
	}
	store, ok := s.store.(operationalStore)
	if !ok {
		return nil, invalid("inspection run store is unavailable")
	}
	return store.ListRuns(ctx, scopeID, limit)
}
func (s *Service) ListFindings(ctx context.Context, scopeID string, limit int) ([]Finding, error) {
	if !allowsScope(ctx, scopeID) {
		return nil, authorization.ErrForbidden
	}
	store, ok := s.store.(operationalStore)
	if !ok {
		return nil, invalid("inspection run store is unavailable")
	}
	return store.ListFindings(ctx, scopeID, limit)
}
func (s *Service) CreateChannel(ctx context.Context, item NotificationChannel) (NotificationChannel, error) {
	if !allowsScope(ctx, item.ScopeID) {
		return NotificationChannel{}, authorization.ErrForbidden
	}
	if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.WebhookURL) == "" {
		return NotificationChannel{}, invalid("channel name and webhook URL are required")
	}
	if item.Status == "" {
		item.Status = "active"
	}
	if item.Status != "active" && item.Status != "disabled" {
		return NotificationChannel{}, invalid("invalid channel status")
	}
	if item.RateLimitPerMinute == 0 {
		item.RateLimitPerMinute = 30
	}
	store, ok := s.store.(operationalStore)
	if !ok {
		return NotificationChannel{}, invalid("notification store is unavailable")
	}
	return store.CreateChannel(ctx, item)
}
func (s *Service) ListChannels(ctx context.Context, scopeID string) ([]NotificationChannel, error) {
	if !allowsScope(ctx, scopeID) {
		return nil, authorization.ErrForbidden
	}
	store, ok := s.store.(operationalStore)
	if !ok {
		return nil, invalid("notification store is unavailable")
	}
	return store.ListChannels(ctx, scopeID)
}
func (s *Service) SetPolicyStatus(ctx context.Context, scopeID, policyID, status string) error {
	if !allowsScope(ctx, scopeID) {
		return authorization.ErrForbidden
	}
	if status != PolicyActive && status != PolicyDisabled {
		return invalid("invalid policy status")
	}
	store, ok := s.store.(interface {
		SetPolicyStatus(context.Context, string, string, string) error
	})
	if !ok {
		return invalid("inspection policy store is unavailable")
	}
	return store.SetPolicyStatus(ctx, policyID, scopeID, status)
}

func (s *Service) validateResource(ctx context.Context, scopeID, id, kind string) error {
	if s.resources == nil {
		return invalid("resource service is unavailable")
	}
	item, err := s.resources.Get(ctx, id)
	if err != nil {
		return err
	}
	if item.ScopeID != scopeID || item.Status != resource.StatusActive {
		return authorization.ErrForbidden
	}
	if kind != "" && item.Kind != kind {
		return invalid("policy AgentProfile is not an AgentProfile resource")
	}
	if filter, ok := authorization.ResourceFilterFromContext(ctx); ok && !filter.Allows(item.ScopeID, item.ID) {
		return authorization.ErrForbidden
	}
	return nil
}

func allowsScope(ctx context.Context, scopeID string) bool {
	filter, ok := authorization.ScopeFilterFromContext(ctx)
	return !ok || filter.Allows(scopeID)
}
