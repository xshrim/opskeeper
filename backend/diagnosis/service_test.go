package diagnosis

import (
	"context"
	"errors"
	"testing"
	"time"

	"opskeeper/backend/authorization"
	"opskeeper/backend/resource"
)

func TestServiceStartRequiresAuthorizedSameScopeTarget(t *testing.T) {
	store := &memoryStore{}
	resources := fakeResources{items: map[string]resource.Resource{
		"target-1": {ID: "target-1", ScopeID: "scope-1", Status: resource.StatusActive},
		"target-2": {ID: "target-2", ScopeID: "scope-2", Status: resource.StatusActive},
	}}
	service := NewService(store, resources)
	ctx := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{ScopeIDs: []string{"scope-1"}, ResourceIDs: []string{"target-1"}})
	item, err := service.Start(ctx, StartInput{ScopeID: "scope-1", ActorUserID: "actor-1", Question: "inspect", TargetResourceIDs: []string{"target-1", "target-1"}})
	if err != nil || item.ID == "" || len(store.started.TargetResourceIDs) != 1 {
		t.Fatalf("Start() = %#v, %v", item, err)
	}
	if _, err := service.Start(ctx, StartInput{ScopeID: "scope-1", ActorUserID: "actor-1", Question: "inspect", TargetResourceIDs: []string{"target-2"}}); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("cross-scope target error = %v", err)
	}
}

func TestServiceAskReopensCompletedSession(t *testing.T) {
	store := &memoryStore{session: Session{ID: "session-1", ScopeID: "scope-1", Status: StatusSucceeded}}
	service := NewService(store, fakeResources{})
	ctx := authorization.WithScopeFilter(context.Background(), authorization.ScopeFilter{ScopeIDs: []string{"scope-1"}})
	message, err := service.Ask(ctx, "session-1", "continue")
	if err != nil || !store.reopened || message.Role != "user" || message.Content != "continue" {
		t.Fatalf("Ask() = %#v, %v; reopened=%v", message, err, store.reopened)
	}
}

type fakeResources struct{ items map[string]resource.Resource }

func (f fakeResources) Get(_ context.Context, id string) (resource.Resource, error) {
	item, ok := f.items[id]
	if !ok {
		return resource.Resource{}, resource.ErrNotFound
	}
	return item, nil
}

type memoryStore struct {
	session  Session
	started  StartInput
	reopened bool
}

func (m *memoryStore) Start(_ context.Context, input StartInput) (Session, error) {
	m.started = input
	m.session = Session{ID: "session-1", ScopeID: input.ScopeID, Status: StatusQueued}
	return m.session, nil
}
func (m *memoryStore) Get(context.Context, string) (Session, error) { return m.session, nil }
func (m *memoryStore) List(context.Context, string, int) ([]Session, error) {
	return []Session{m.session}, nil
}
func (*memoryStore) Delete(context.Context, string) error                      { return nil }
func (*memoryStore) Targets(context.Context, string) ([]Target, error)         { return nil, nil }
func (*memoryStore) AddTarget(context.Context, string, string) (Target, error) { return Target{}, nil }
func (*memoryStore) Messages(context.Context, string, int) ([]Message, error)  { return nil, nil }
func (*memoryStore) AppendMessage(_ context.Context, id string, input AppendMessageInput) (Message, error) {
	return Message{ID: "message-1", SessionID: id, Role: input.Role, Content: input.Content, CreatedAt: time.Now()}, nil
}
func (*memoryStore) CreatePlan(context.Context, string, string, []PlanStep) (Plan, error) {
	return Plan{}, nil
}
func (*memoryStore) Plan(context.Context, string) (Plan, error) { return Plan{}, ErrNotFound }
func (*memoryStore) UpdateStep(context.Context, string, string, string) (PlanStep, error) {
	return PlanStep{}, nil
}
func (*memoryStore) AppendEvent(context.Context, string, CreateEventInput) (Event, error) {
	return Event{}, nil
}
func (*memoryStore) EventsAfter(context.Context, string, int64, int) ([]Event, error) {
	return nil, nil
}
func (*memoryStore) SaveEvidence(context.Context, string, CreateEvidenceInput) (Evidence, error) {
	return Evidence{}, nil
}
func (*memoryStore) Evidence(context.Context, string) ([]Evidence, error) { return nil, nil }
func (*memoryStore) CreateRun(context.Context, string, string) (Run, error) {
	return Run{ID: "run-1", Status: "running"}, nil
}
func (*memoryStore) FinishRun(context.Context, string, string) (Run, error) { return Run{}, nil }
func (*memoryStore) Runs(context.Context, string) ([]Run, error)            { return nil, nil }
func (*memoryStore) SaveCausalChain(context.Context, CausalChain) (CausalChain, error) {
	return CausalChain{}, nil
}
func (*memoryStore) CausalChains(context.Context, string) ([]CausalChain, error) { return nil, nil }
func (*memoryStore) SaveHypothesis(context.Context, Hypothesis) (Hypothesis, error) {
	return Hypothesis{}, nil
}
func (*memoryStore) Hypotheses(context.Context, string) ([]Hypothesis, error) { return nil, nil }
func (*memoryStore) SaveReport(context.Context, Report) (Report, error)       { return Report{}, nil }
func (*memoryStore) Report(context.Context, string) (Report, error)           { return Report{}, ErrNotFound }
func (m *memoryStore) SetStatus(context.Context, string, Status) (Session, error) {
	return m.session, nil
}
func (m *memoryStore) ClaimRun(context.Context, string) (Session, bool, error) {
	m.session.Status = StatusPlanning
	return m.session, true, nil
}
func (m *memoryStore) Reopen(context.Context, string) (Session, error) {
	m.reopened = true
	m.session.Status = StatusQueued
	return m.session, nil
}
func (*memoryStore) Finish(context.Context, string, Status, string, string) (Session, error) {
	return Session{}, nil
}
