package diagnosis

import (
	"context"
	"errors"
	"slices"
	"strings"

	"opskeeper/backend/authorization"
	"opskeeper/backend/resource"
)

type ResourceReader interface {
	Get(context.Context, string) (resource.Resource, error)
}

type Service struct {
	store     Store
	resources ResourceReader
}

func NewService(store Store, resources ResourceReader) *Service {
	return &Service{store: store, resources: resources}
}

func (s *Service) Start(ctx context.Context, input StartInput) (Session, error) {
	input.ScopeID = strings.TrimSpace(input.ScopeID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Title = strings.TrimSpace(input.Title)
	input.Question = strings.TrimSpace(input.Question)
	input.ProviderResourceID = strings.TrimSpace(input.ProviderResourceID)
	input.ModelName = strings.TrimSpace(input.ModelName)
	if input.ScopeID == "" || input.ActorUserID == "" || input.Question == "" {
		return Session{}, invalid("scope_id, actor_user_id and question are required")
	}
	if len([]rune(input.Title)) > 200 || len([]rune(input.Question)) > 16000 {
		return Session{}, invalid("diagnosis title or question is too long")
	}
	input.TargetResourceIDs = distinctIDs(input.TargetResourceIDs)
	if len(input.TargetResourceIDs) > 20 {
		return Session{}, invalid("a session may load at most 20 context resources")
	}
	if !allowsScope(ctx, input.ScopeID) {
		return Session{}, authorization.ErrForbidden
	}
	for _, resourceID := range input.TargetResourceIDs {
		if err := s.validateTarget(ctx, input.ScopeID, resourceID); err != nil {
			return Session{}, err
		}
	}
	if input.Title == "" {
		input.Title = titleFromQuestion(input.Question)
	}
	return s.store.Start(ctx, input)
}

func (s *Service) Get(ctx context.Context, sessionID string) (Snapshot, error) {
	session, err := s.store.Get(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return Snapshot{}, err
	}
	if !allowsScope(ctx, session.ScopeID) {
		return Snapshot{}, authorization.ErrForbidden
	}
	targets, err := s.store.Targets(ctx, session.ID)
	if err != nil {
		return Snapshot{}, err
	}
	for _, target := range targets {
		if err := s.validateTarget(ctx, session.ScopeID, target.ResourceID); err != nil {
			return Snapshot{}, err
		}
	}
	messages, err := s.store.Messages(ctx, session.ID, 100)
	if err != nil {
		return Snapshot{}, err
	}
	evidence, err := s.store.Evidence(ctx, session.ID)
	if err != nil {
		return Snapshot{}, err
	}
	hypotheses, err := s.store.Hypotheses(ctx, session.ID)
	if err != nil {
		return Snapshot{}, err
	}
	events, err := s.store.EventsAfter(ctx, session.ID, 0, 500)
	if err != nil {
		return Snapshot{}, err
	}
	result := Snapshot{Session: session, Targets: targets, Messages: messages, Evidence: evidence, Hypotheses: hypotheses, Events: events}
	if plan, err := s.store.Plan(ctx, session.ID); err == nil {
		result.Plan = &plan
	} else if !errors.Is(err, ErrNotFound) {
		return Snapshot{}, err
	}
	if report, err := s.store.Report(ctx, session.ID); err == nil {
		result.Report = &report
	} else if !errors.Is(err, ErrNotFound) {
		return Snapshot{}, err
	}
	return result, nil
}

func (s *Service) List(ctx context.Context, scopeID string, limit int) ([]Session, error) {
	scopeID = strings.TrimSpace(scopeID)
	if !allowsScope(ctx, scopeID) {
		return nil, authorization.ErrForbidden
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.store.List(ctx, scopeID, limit)
}

func (s *Service) Delete(ctx context.Context, sessionID string) error {
	session, err := s.store.Get(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return err
	}
	if !allowsScope(ctx, session.ScopeID) {
		return authorization.ErrForbidden
	}
	return s.store.Delete(ctx, session.ID)
}

func (s *Service) AddTarget(ctx context.Context, sessionID, resourceID string) (Target, error) {
	session, err := s.store.Get(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return Target{}, err
	}
	if session.Status == StatusSucceeded || session.Status == StatusFailed || session.Status == StatusCancelled {
		return Target{}, ErrConflict
	}
	if err := s.validateTarget(ctx, session.ScopeID, strings.TrimSpace(resourceID)); err != nil {
		return Target{}, err
	}
	item, err := s.store.AddTarget(ctx, session.ID, strings.TrimSpace(resourceID))
	if err != nil {
		return Target{}, err
	}
	_, _ = s.store.AppendEvent(ctx, session.ID, CreateEventInput{Type: "target.added", Payload: map[string]any{"resource_id": item.ResourceID}})
	return item, nil
}

func (s *Service) Ask(ctx context.Context, sessionID, content string) (Message, error) {
	session, err := s.store.Get(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return Message{}, err
	}
	if !allowsScope(ctx, session.ScopeID) {
		return Message{}, authorization.ErrForbidden
	}
	if session.Status == StatusSucceeded || session.Status == StatusFailed || session.Status == StatusCancelled {
		if _, err := s.store.Reopen(ctx, session.ID); err != nil {
			return Message{}, err
		}
	}
	content = strings.TrimSpace(content)
	if content == "" || len([]rune(content)) > 16000 {
		return Message{}, invalid("message must contain 1 to 16000 characters")
	}
	item, err := s.store.AppendMessage(ctx, session.ID, AppendMessageInput{Role: "user", Content: content})
	if err != nil {
		return Message{}, err
	}
	_, _ = s.store.AppendEvent(ctx, session.ID, CreateEventInput{Type: "message.created", Payload: map[string]any{"message_id": item.ID, "role": item.Role}})
	return item, nil
}

func (s *Service) EventsAfter(ctx context.Context, sessionID string, after int64, limit int) ([]Event, error) {
	session, err := s.store.Get(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, err
	}
	if !allowsScope(ctx, session.ScopeID) {
		return nil, authorization.ErrForbidden
	}
	if after < 0 {
		return nil, invalid("event id must not be negative")
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	return s.store.EventsAfter(ctx, session.ID, after, limit)
}

func (s *Service) validateTarget(ctx context.Context, scopeID, resourceID string) error {
	if resourceID == "" || s.resources == nil {
		return invalid("target_resource_id is required")
	}
	item, err := s.resources.Get(ctx, resourceID)
	if err != nil {
		return err
	}
	if item.Status == resource.StatusDisabled {
		return invalid("target resource is disabled")
	}
	if item.ScopeID != scopeID || !resourceVisible(ctx, item.ScopeID, item.ID) {
		return authorization.ErrForbidden
	}
	return nil
}

func allowsScope(ctx context.Context, scopeID string) bool {
	filter, ok := authorization.ScopeFilterFromContext(ctx)
	return !ok || filter.Allows(scopeID)
}

func resourceVisible(ctx context.Context, scopeID, resourceID string) bool {
	filter, ok := authorization.ResourceFilterFromContext(ctx)
	return !ok || filter.Allows(scopeID, resourceID)
}

func distinctIDs(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !slices.Contains(result, value) {
			result = append(result, value)
		}
	}
	return result
}

func titleFromQuestion(question string) string {
	runes := []rune(question)
	if len(runes) > 80 {
		return string(runes[:80]) + "…"
	}
	return question
}
