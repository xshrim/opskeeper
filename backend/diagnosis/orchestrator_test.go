package diagnosis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"opskeeper/backend/aiengine"
)

func TestOrchestratorCreatesTraceableReportFromObservedEvidence(t *testing.T) {
	store := newRecordingStore()
	engine := fakeEngine{execute: func(_ context.Context, request aiengine.Request) (aiengine.Result, error) {
		request.ObservationSink(aiengine.ToolObservation{ToolName: "connector_metrics_query", ResourceID: "target-1", Result: aiengine.ToolResult{Output: map[string]any{"rate": 0.12}, Untrusted: true}})
		return aiengine.Result{Output: "错误率在采样窗口内升高。"}, nil
	}}
	orchestrator := NewOrchestrator(&Service{store: store}, engine, time.Second)
	orchestrator.run(context.Background(), "session-1")

	if store.session.Status != StatusSucceeded {
		t.Fatalf("status = %q, want succeeded", store.session.Status)
	}
	if len(store.evidence) != 1 || store.evidence[0].ContentHash == "" || !store.evidence[0].Untrusted {
		t.Fatalf("evidence = %#v, want persisted untrusted evidence with hash", store.evidence)
	}
	if store.report.Status != "succeeded" || len(store.report.EvidenceIDs) != 1 || store.report.EvidenceIDs[0] != store.evidence[0].ID {
		t.Fatalf("report = %#v, want evidence-backed report", store.report)
	}
	if len(store.hypotheses) != 1 || store.hypotheses[0].Status != "supported" || len(store.hypotheses[0].EvidenceIDs) != 1 {
		t.Fatalf("hypotheses = %#v, want supported hypothesis", store.hypotheses)
	}
	if !store.hasEvent("evidence.collected") || !store.hasEvent("report.ready") {
		t.Fatalf("events = %#v, want evidence and report events", store.events)
	}
}

func TestOrchestratorMarksOutputWithoutEvidenceAsNeedsVerification(t *testing.T) {
	store := newRecordingStore()
	orchestrator := NewOrchestrator(&Service{store: store}, fakeEngine{execute: func(context.Context, aiengine.Request) (aiengine.Result, error) {
		return aiengine.Result{Output: "可能是上游依赖变慢。"}, nil
	}}, time.Second)
	orchestrator.run(context.Background(), "session-1")

	if store.session.Status != StatusSucceeded || store.report.Status != "warning" || len(store.report.EvidenceIDs) != 0 {
		t.Fatalf("session/report = %#v / %#v", store.session, store.report)
	}
	if len(store.hypotheses) != 1 || store.hypotheses[0].Status != "needs_verification" || store.hypotheses[0].Confidence != 0 {
		t.Fatalf("hypothesis = %#v, want unverified", store.hypotheses)
	}
}

func TestOrchestratorFailureAndConcurrentClaimDoNotLeaveActiveSession(t *testing.T) {
	store := newRecordingStore()
	orchestrator := NewOrchestrator(&Service{store: store}, fakeEngine{execute: func(context.Context, aiengine.Request) (aiengine.Result, error) {
		return aiengine.Result{}, errors.New("provider unavailable")
	}}, time.Second)
	orchestrator.run(context.Background(), "session-1")
	if store.session.Status != StatusFailed || !store.hasEvent("diagnosis.failed") {
		t.Fatalf("failed run left session = %#v, events=%#v", store.session, store.events)
	}

	store = newRecordingStore()
	store.claimed = true
	calls := 0
	orchestrator = NewOrchestrator(&Service{store: store}, fakeEngine{execute: func(context.Context, aiengine.Request) (aiengine.Result, error) {
		calls++
		return aiengine.Result{}, nil
	}}, time.Second)
	orchestrator.run(context.Background(), "session-1")
	if calls != 0 || store.session.Status != StatusQueued {
		t.Fatalf("unclaimed session invoked runner=%d and status=%q", calls, store.session.Status)
	}
}

type fakeEngine struct {
	execute func(context.Context, aiengine.Request) (aiengine.Result, error)
}

func (f fakeEngine) Name() string { return "fake" }
func (f fakeEngine) Execute(ctx context.Context, input aiengine.Request) (aiengine.Result, error) {
	return f.execute(ctx, input)
}
func (f fakeEngine) Stream(context.Context, aiengine.Request) (<-chan aiengine.Event, error) {
	return nil, errors.New("stream not implemented")
}
func (f fakeEngine) Cancel(context.Context, string) error { return nil }

type recordingStore struct {
	mu         sync.Mutex
	session    Session
	targets    []Target
	messages   []Message
	plan       Plan
	evidence   []Evidence
	hypotheses []Hypothesis
	report     Report
	events     []Event
	claimed    bool
}

func newRecordingStore() *recordingStore {
	actor := "actor-1"
	return &recordingStore{session: Session{ID: "session-1", ScopeID: "scope-1", ActorUserID: &actor, Status: StatusQueued}, targets: []Target{{SessionID: "session-1", ResourceID: "target-1"}}, messages: []Message{{ID: "message-1", SessionID: "session-1", Role: "user", Content: "检查错误率"}}}
}

func (s *recordingStore) Start(context.Context, StartInput) (Session, error) { return s.session, nil }
func (s *recordingStore) Get(context.Context, string) (Session, error)       { return s.session, nil }
func (s *recordingStore) List(context.Context, string, int) ([]Session, error) {
	return []Session{s.session}, nil
}
func (s *recordingStore) Targets(context.Context, string) ([]Target, error) { return s.targets, nil }
func (s *recordingStore) AddTarget(context.Context, string, string) (Target, error) {
	return Target{}, nil
}
func (s *recordingStore) Messages(context.Context, string, int) ([]Message, error) {
	return s.messages, nil
}
func (s *recordingStore) AppendMessage(_ context.Context, sessionID string, input AppendMessageInput) (Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	message := Message{ID: fmt.Sprintf("message-%d", len(s.messages)+1), SessionID: sessionID, Role: input.Role, Content: input.Content, CreatedAt: time.Now()}
	s.messages = append(s.messages, message)
	return message, nil
}
func (s *recordingStore) CreatePlan(_ context.Context, sessionID, summary string, steps []PlanStep) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range steps {
		steps[index].ID = fmt.Sprintf("step-%d", index+1)
		steps[index].PlanID = "plan-1"
		steps[index].Sequence = index + 1
	}
	s.plan = Plan{ID: "plan-1", SessionID: sessionID, Summary: summary, Steps: steps}
	return s.plan, nil
}
func (s *recordingStore) Plan(context.Context, string) (Plan, error) {
	if s.plan.ID == "" {
		return Plan{}, ErrNotFound
	}
	return s.plan, nil
}
func (s *recordingStore) UpdateStep(_ context.Context, id, status, detail string) (PlanStep, error) {
	for index := range s.plan.Steps {
		if s.plan.Steps[index].ID == id {
			s.plan.Steps[index].Status, s.plan.Steps[index].Detail = status, detail
			return s.plan.Steps[index], nil
		}
	}
	return PlanStep{}, ErrNotFound
}
func (s *recordingStore) AppendEvent(_ context.Context, sessionID string, input CreateEventInput) (Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	payload, _ := json.Marshal(input.Payload)
	event := Event{ID: int64(len(s.events) + 1), SessionID: sessionID, Type: input.Type, Payload: payload}
	s.events = append(s.events, event)
	return event, nil
}
func (s *recordingStore) EventsAfter(context.Context, string, int64, int) ([]Event, error) {
	return s.events, nil
}
func (s *recordingStore) SaveEvidence(_ context.Context, sessionID string, input CreateEvidenceInput) (Evidence, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := Evidence{ID: fmt.Sprintf("evidence-%d", len(s.evidence)+1), SessionID: sessionID, TargetResourceID: pointer(input.TargetResourceID), SourceResourceID: pointer(input.SourceResourceID), Capability: input.Capability, CollectedAt: input.CollectedAt, ContentHash: "hashed", Content: input.Content, Partial: input.Partial, Untrusted: input.Untrusted}
	s.evidence = append(s.evidence, item)
	return item, nil
}
func (s *recordingStore) Evidence(context.Context, string) ([]Evidence, error) {
	return s.evidence, nil
}
func (s *recordingStore) SaveHypothesis(_ context.Context, input Hypothesis) (Hypothesis, error) {
	input.ID = fmt.Sprintf("hypothesis-%d", len(s.hypotheses)+1)
	s.hypotheses = append(s.hypotheses, input)
	return input, nil
}
func (s *recordingStore) Hypotheses(context.Context, string) ([]Hypothesis, error) {
	return s.hypotheses, nil
}
func (s *recordingStore) SaveReport(_ context.Context, input Report) (Report, error) {
	input.ID = "report-1"
	s.report = input
	return input, nil
}
func (s *recordingStore) Report(context.Context, string) (Report, error) {
	if s.report.ID == "" {
		return Report{}, ErrNotFound
	}
	return s.report, nil
}
func (s *recordingStore) ClaimRun(context.Context, string) (Session, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claimed {
		return Session{}, false, nil
	}
	s.claimed = true
	s.session.Status = StatusPlanning
	return s.session, true, nil
}
func (s *recordingStore) SetStatus(_ context.Context, _ string, status Status) (Session, error) {
	s.session.Status = status
	return s.session, nil
}
func (s *recordingStore) Reopen(context.Context, string) (Session, error) {
	s.session.Status = StatusQueued
	return s.session, nil
}
func (s *recordingStore) Finish(_ context.Context, _ string, status Status, code, message string) (Session, error) {
	s.session.Status, s.session.ErrorCode, s.session.ErrorMessage = status, code, message
	return s.session, nil
}
func (s *recordingStore) hasEvent(kind string) bool {
	for _, event := range s.events {
		if event.Type == kind {
			return true
		}
	}
	return false
}
func pointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
