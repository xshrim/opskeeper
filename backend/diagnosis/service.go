package diagnosis

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"

	"opskeeper/backend/application"
	"opskeeper/backend/authorization"
	"opskeeper/backend/resource"
)

type ResourceReader interface {
	Get(context.Context, string) (resource.Resource, error)
}

type ApplicationReader interface {
	Get(context.Context, string) (application.Application, error)
	ContextResourceIDs(context.Context, string, string) ([]string, error)
	DiagnosticContext(context.Context, string, string) (application.DiagnosticContext, error)
}

type Service struct {
	store        Store
	resources    ResourceReader
	applications ApplicationReader
	eventMu      sync.Mutex
	subMu        sync.Mutex
	subscribers  map[string]map[chan struct{}]struct{}
}

func NewService(store Store, resources ResourceReader, applications ApplicationReader) *Service {
	return &Service{store: store, resources: resources, applications: applications, subscribers: make(map[string]map[chan struct{}]struct{})}
}

func (s *Service) Start(ctx context.Context, input StartInput) (Session, error) {
	input.ScopeID = strings.TrimSpace(input.ScopeID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Title = strings.TrimSpace(input.Title)
	input.Question = strings.TrimSpace(input.Question)
	input.ApplicationID = strings.TrimSpace(input.ApplicationID)
	input.ApplicationIDs = distinctIDs(input.ApplicationIDs)
	if len(input.ApplicationIDs) == 0 && input.ApplicationID != "" {
		input.ApplicationIDs = []string{input.ApplicationID}
	}
	associatedTargetSet := make(map[string]struct{})
	if len(input.ApplicationIDs) > 0 {
		input.ApplicationID = input.ApplicationIDs[0]
	}
	input.ProviderID = strings.TrimSpace(input.ProviderID)
	input.ModelName = strings.TrimSpace(input.ModelName)
	input.SkillID = strings.TrimSpace(input.SkillID)
	input.PersonaID = strings.TrimSpace(input.PersonaID)
	input.Links = distinctStrings(input.Links)
	if len(input.Attachments) > 10 || len(input.Links) > 10 {
		return Session{}, invalid("a message may include at most 10 links or attachments")
	}
	if err := validateMessageExtras(input.Attachments, input.Links); err != nil {
		return Session{}, err
	}
	if input.ScopeID == "" || input.ActorUserID == "" || input.Question == "" {
		return Session{}, invalid("scope_id, actor_user_id and question are required")
	}
	if len([]rune(input.Title)) > 200 || len([]rune(input.Question)) > 16000 {
		return Session{}, invalid("diagnosis title or question is too long")
	}
	if len(input.ApplicationIDs) > 0 {
		if len(input.ApplicationIDs) > 20 {
			return Session{}, invalid("a session may include at most 20 applications")
		}
		if s.applications == nil {
			return Session{}, invalid("application context is unavailable")
		}
		contexts := make([]application.DiagnosticContext, 0, len(input.ApplicationIDs))
		associatedTargetIDs := make([]string, 0)
		for _, applicationID := range input.ApplicationIDs {
			app, err := s.applications.Get(ctx, applicationID)
			if err != nil {
				return Session{}, err
			}
			if app.Status == "disabled" {
				return Session{}, invalid("application is disabled")
			}
			if app.ProjectScopeID != "" && !allowsScope(ctx, app.ProjectScopeID) {
				return Session{}, authorization.ErrForbidden
			}
			applicationScopeID := input.ScopeID
			if app.ProjectScopeID != "" {
				applicationScopeID = app.ProjectScopeID
			}
			applicationContext, contextErr := s.applications.DiagnosticContext(ctx, applicationID, applicationScopeID)
			if contextErr != nil {
				return Session{}, contextErr
			}
			contexts = append(contexts, applicationContext)
			associated, err := s.applications.ContextResourceIDs(ctx, applicationID, applicationScopeID)
			if err != nil {
				return Session{}, err
			}
			// An application may reference resources the caller cannot use. The
			// application itself remains selectable, but only its visible, active
			// resources become diagnosis targets.
			for _, resourceID := range associated {
				item, getErr := s.resources.Get(ctx, resourceID)
				if getErr != nil {
					if errors.Is(getErr, resource.ErrNotFound) || errors.Is(getErr, authorization.ErrForbidden) {
						continue
					}
					return Session{}, getErr
				}
				if !applicationResourceVisible(ctx, item.ScopeID, item.ID) {
					continue
				}
				associatedTargetIDs = append(associatedTargetIDs, resourceID)
				associatedTargetSet[resourceID] = struct{}{}
			}
		}
		applicationContextJSON, err := json.Marshal(struct {
			Applications []application.DiagnosticContext `json:"applications"`
		}{Applications: contexts})
		if err != nil {
			return Session{}, invalid("application context is not serializable")
		}
		input.ApplicationContext = applicationContextJSON
		input.TargetResourceIDs = append(associatedTargetIDs, input.TargetResourceIDs...)
	}
	input.TargetResourceIDs = distinctIDs(input.TargetResourceIDs)
	if len(input.TargetResourceIDs) > 20 {
		return Session{}, invalid("a session may load at most 20 context resources")
	}
	if !allowsScope(ctx, input.ScopeID) {
		return Session{}, authorization.ErrForbidden
	}
	for _, resourceID := range input.TargetResourceIDs {
		if _, associated := associatedTargetSet[resourceID]; associated {
			continue
		}
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
	if err := ensureSessionAccess(ctx, session); err != nil {
		return Snapshot{}, err
	}
	if sessionOwnedByActor(ctx, session) {
		if err := s.validateSessionTargets(ctx, session); err != nil {
			return Snapshot{}, err
		}
	}
	targets, err := s.store.Targets(ctx, session.ID)
	if err != nil {
		return Snapshot{}, err
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
	// Keep enough history for long-running Agentic loops. The SSE endpoint
	// remains the live transport; this snapshot bound is only the reconnect and
	// page-reload baseline and must not silently discard earlier iterations.
	events, err := s.store.EventsAfter(ctx, session.ID, 0, 5000)
	if err != nil {
		return Snapshot{}, err
	}
	runs, err := s.store.Runs(ctx, session.ID)
	if err != nil {
		return Snapshot{}, err
	}
	chains, err := s.store.CausalChains(ctx, session.ID)
	if err != nil {
		return Snapshot{}, err
	}
	result := Snapshot{Session: session, Targets: targets, Messages: messages, Evidence: evidence, Runs: runs, CausalChains: chains, Hypotheses: hypotheses, Events: events}
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
	return s.store.List(ctx, scopeID, diagnosisActorID(ctx), limit)
}

func (s *Service) Delete(ctx context.Context, sessionID string) error {
	session, err := s.store.Get(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return err
	}
	if err := ensureSessionOwner(ctx, session); err != nil {
		return err
	}
	return s.store.Delete(ctx, session.ID)
}

func (s *Service) AddTarget(ctx context.Context, sessionID, resourceID string) (Target, error) {
	session, err := s.store.Get(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return Target{}, err
	}
	if err := ensureSessionOwner(ctx, session); err != nil {
		return Target{}, err
	}
	if session.Status == StatusSucceeded || session.Status == StatusFailed || session.Status == StatusCancelled {
		if _, err := s.store.Reopen(ctx, session.ID); err != nil {
			return Target{}, err
		}
	}
	if err := s.validateTarget(ctx, session.ScopeID, strings.TrimSpace(resourceID)); err != nil {
		return Target{}, err
	}
	item, err := s.store.AddTarget(ctx, session.ID, strings.TrimSpace(resourceID))
	if err != nil {
		return Target{}, err
	}
	// The target is persisted independently; event delivery is best-effort.
	_ = s.appendEvent(ctx, session.ID, CreateEventInput{Type: "target.added", Payload: map[string]any{"resource_id": item.ResourceID}})
	return item, nil
}

func (s *Service) Ask(ctx context.Context, sessionID, content string) (Message, error) {
	return s.AskMessage(ctx, sessionID, AppendMessageInput{Content: content})
}

func (s *Service) AskMessage(ctx context.Context, sessionID string, input AppendMessageInput) (Message, error) {
	session, err := s.store.Get(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return Message{}, err
	}
	if err := ensureSessionOwner(ctx, session); err != nil {
		return Message{}, err
	}
	if err := s.validateSessionTargets(ctx, session); err != nil {
		return Message{}, err
	}
	if session.Status == StatusSucceeded || session.Status == StatusFailed || session.Status == StatusCancelled {
		if _, err := s.store.Reopen(ctx, session.ID); err != nil {
			return Message{}, err
		}
	}
	input.Content = strings.TrimSpace(input.Content)
	input.SkillID = strings.TrimSpace(input.SkillID)
	input.PersonaID = strings.TrimSpace(input.PersonaID)
	input.Links = distinctStrings(input.Links)
	if len(input.Attachments) > 10 || len(input.Links) > 10 {
		return Message{}, invalid("a message may include at most 10 links or attachments")
	}
	if err := validateMessageExtras(input.Attachments, input.Links); err != nil {
		return Message{}, err
	}
	if input.Content == "" || len([]rune(input.Content)) > 16000 {
		return Message{}, invalid("message must contain 1 to 16000 characters")
	}
	input.Role = "user"
	item, err := s.store.AppendMessage(ctx, session.ID, input)
	if err != nil {
		return Message{}, err
	}
	_ = s.appendEvent(ctx, session.ID, CreateEventInput{Type: "message.created", Payload: map[string]any{"message_id": item.ID, "role": item.Role}})
	return item, nil
}

func validateMessageExtras(attachments []Attachment, links []string) error {
	totalBytes := 0
	for _, attachment := range attachments {
		if strings.TrimSpace(attachment.Name) == "" || strings.TrimSpace(attachment.Content) == "" {
			return invalid("attachment name and text content are required")
		}
		totalBytes += len(attachment.Content)
	}
	if totalBytes > 128*1024 {
		return invalid("text attachments may not exceed 128 KiB per message")
	}
	for _, link := range links {
		if !strings.HasPrefix(link, "https://") && !strings.HasPrefix(link, "http://") {
			return invalid("links must use http or https")
		}
	}
	return nil
}

func distinctStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// appendEvent is shared by the user-facing Service mutations and the
// Orchestrator's background Agentic loop. A follow-up message or target can
// arrive while tools are completing, so all diagnosis timeline writes must
// use the same lock before obtaining the database event ID.
func (s *Service) appendEvent(ctx context.Context, sessionID string, input CreateEventInput) error {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	_, err := s.store.AppendEvent(ctx, sessionID, input)
	if err == nil {
		s.notifyEventSubscribers(sessionID)
	}
	return err
}

// SubscribeEvents wakes an SSE consumer immediately after an event has been
// durably appended. The channel is only a wake-up hint; consumers must still
// use their event cursor to fetch the authoritative ordered rows.
func (s *Service) SubscribeEvents(ctx context.Context, sessionID string) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	subID := strings.TrimSpace(sessionID)
	s.subMu.Lock()
	if s.subscribers == nil {
		s.subscribers = make(map[string]map[chan struct{}]struct{})
	}
	if s.subscribers[subID] == nil {
		s.subscribers[subID] = make(map[chan struct{}]struct{})
	}
	s.subscribers[subID][ch] = struct{}{}
	s.subMu.Unlock()
	var removeOnce sync.Once
	done := make(chan struct{})
	remove := func() {
		removeOnce.Do(func() {
			close(done)
			s.subMu.Lock()
			if group := s.subscribers[subID]; group != nil {
				delete(group, ch)
				if len(group) == 0 {
					delete(s.subscribers, subID)
				}
			}
			s.subMu.Unlock()
		})
	}
	if ctx != nil {
		go func() {
			select {
			case <-ctx.Done():
				remove()
			case <-done:
			}
		}()
	}
	return ch, remove
}

func (s *Service) notifyEventSubscribers(sessionID string) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for ch := range s.subscribers[strings.TrimSpace(sessionID)] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (s *Service) EventsAfter(ctx context.Context, sessionID string, after int64, limit int) ([]Event, error) {
	session, err := s.store.Get(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, err
	}
	if err := ensureSessionAccess(ctx, session); err != nil {
		return nil, err
	}
	if after < 0 {
		return nil, invalid("event id must not be negative")
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	return s.store.EventsAfter(ctx, session.ID, after, limit)
}

// SetScopeShared lets only the session creator opt in or out of sharing the
// entire diagnosis with users that have diagnosis:read in the same scope.
func (s *Service) SetScopeShared(ctx context.Context, sessionID string, shared bool) (Session, error) {
	session, err := s.store.Get(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return Session{}, err
	}
	if err := ensureSessionOwner(ctx, session); err != nil {
		return Session{}, err
	}
	return s.store.SetScopeShared(ctx, session.ID, shared)
}

func (s *Service) validateTarget(ctx context.Context, scopeID, resourceID string) error {
	if resourceID == "" || s.resources == nil {
		return invalid("target_resource_id is required")
	}
	item, err := s.resources.Get(ctx, resourceID)
	if err != nil {
		return err
	}
	if !resourceVisible(ctx, item.ScopeID, item.ID) {
		return authorization.ErrForbidden
	}
	return nil
}

func (s *Service) validateSessionTargets(ctx context.Context, session Session) error {
	targets, err := s.store.Targets(ctx, session.ID)
	if err != nil {
		return err
	}
	for _, target := range targets {
		if err := s.validateTarget(ctx, session.ScopeID, target.ResourceID); err != nil {
			return err
		}
	}
	return nil
}

func allowsScope(ctx context.Context, scopeID string) bool {
	filter, ok := authorization.ScopeFilterFromContext(ctx)
	return !ok || filter.Allows(scopeID)
}

func diagnosisActorID(ctx context.Context) string {
	if filter, ok := authorization.ResourceFilterFromContext(ctx); ok {
		return strings.TrimSpace(filter.SubjectID)
	}
	if filter, ok := authorization.ScopeFilterFromContext(ctx); ok {
		return strings.TrimSpace(filter.SubjectID)
	}
	return ""
}

func ensureSessionAccess(ctx context.Context, session Session) error {
	if !allowsScope(ctx, session.ScopeID) {
		return authorization.ErrForbidden
	}
	if sessionOwnedByActor(ctx, session) {
		return nil
	}
	if session.ScopeShared {
		return nil
	}
	return authorization.ErrForbidden
}

func sessionOwnedByActor(ctx context.Context, session Session) bool {
	actorID := diagnosisActorID(ctx)
	return actorID != "" && session.ActorUserID != nil && strings.TrimSpace(*session.ActorUserID) == actorID
}

func ensureSessionOwner(ctx context.Context, session Session) error {
	if !allowsScope(ctx, session.ScopeID) || !sessionOwnedByActor(ctx, session) {
		return authorization.ErrForbidden
	}
	return nil
}

func resourceVisible(ctx context.Context, scopeID, resourceID string) bool {
	filter, ok := authorization.ResourceUseFilterFromContext(ctx)
	if !ok {
		filter, ok = authorization.ResourceFilterFromContext(ctx)
	}
	return !ok || filter.Allows(scopeID, resourceID)
}

func applicationResourceVisible(ctx context.Context, resourceScopeID, resourceID string) bool {
	return resourceVisible(ctx, resourceScopeID, resourceID)
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
