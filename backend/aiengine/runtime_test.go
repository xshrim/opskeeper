package aiengine

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRunner struct {
	run func(context.Context, Request) (Result, error)
}

type fakePlanResolver struct{ calls int }

func (f *fakePlanResolver) ResolvePlan(context.Context, string, string, string) (ExecutionPlan, error) {
	f.calls++
	return ExecutionPlan{SourceResourceID: "skill-1", SourceVersionID: "version-1", Instruction: "plan instruction"}, nil
}

func TestRuntimeResolvesPlanBeforeAgentRunner(t *testing.T) {
	plan := &fakePlanResolver{}
	var received Request
	runtime := New(fakeRunner{run: func(context.Context, Request) (Result, error) {
		return Result{Output: "generic"}, nil
	}})
	runtime.runner = fakeRunner{run: func(_ context.Context, request Request) (Result, error) {
		received = request
		return Result{Output: "generic"}, nil
	}}
	runtime.WithPlanResolver(plan)
	result, err := runtime.Execute(context.Background(), Request{ScopeID: "scope-1", SkillResourceID: "skill-1", Task: "inspect"})
	if err != nil || result.Output != "generic" || plan.calls != 1 || received.Instruction != "plan instruction" {
		t.Fatalf("plan resolution result=%+v err=%v calls=%d request=%+v", result, err, plan.calls, received)
	}
}

func (f fakeRunner) Run(ctx context.Context, request Request) (Result, error) {
	return f.run(ctx, request)
}

func TestRequestNormalizeDefaultsBudgetAndProfile(t *testing.T) {
	request := Request{ScopeID: "scope-1", Task: "inspect"}
	if err := request.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if request.Profile != ProfileInteractive || request.Budget.MaxToolCalls != 12 || request.Budget.Timeout != 2*time.Minute {
		t.Fatalf("unexpected defaults: %+v", request)
	}
}

func TestRuntimeExecuteEmitsLifecycleAndUsesExecutionID(t *testing.T) {
	var events []Event
	runtime := New(fakeRunner{run: func(_ context.Context, request Request) (Result, error) {
		if request.EventSink == nil {
			t.Fatal("runner did not receive an event sink")
		}
		_ = request.EventSink(Event{Type: "assistant.completed", Payload: map[string]any{"text": "ok"}})
		return Result{Output: "ok", TotalTokens: 3}, nil
	}})
	result, err := runtime.Execute(context.Background(), Request{ExecutionID: "exec-1", ScopeID: "scope-1", Task: "inspect", EventSink: func(event Event) error {
		events = append(events, event)
		return nil
	}})
	if err != nil || result.ExecutionID != "exec-1" || result.Status != StatusSucceeded {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if len(events) != 3 || events[0].Type != "execution.started" || events[1].Type != "assistant.completed" || events[2].Type != "execution.completed" {
		t.Fatalf("unexpected lifecycle events: %+v", events)
	}
	if events[0].Sequence != 1 || events[2].Sequence != 3 {
		t.Fatalf("unexpected event sequence: %+v", events)
	}
}

func TestRuntimeEventSequenceIsMonotonic(t *testing.T) {
	var events []Event
	runtime := New(fakeRunner{run: func(_ context.Context, request Request) (Result, error) {
		_ = request.EventSink(Event{Type: "tool.started", Sequence: 1})
		_ = request.EventSink(Event{Type: "tool.completed", Sequence: 1})
		return Result{}, nil
	}})
	if _, err := runtime.Execute(context.Background(), Request{ScopeID: "scope-1", Task: "inspect", EventSink: func(event Event) error {
		events = append(events, event)
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	for index := 1; index < len(events); index++ {
		if events[index].Sequence <= events[index-1].Sequence {
			t.Fatalf("event sequence is not increasing: %+v", events)
		}
	}
}

func TestRuntimeEmitsFailedLifecycleEventForFailedResult(t *testing.T) {
	var events []Event
	runtime := New(fakeRunner{run: func(context.Context, Request) (Result, error) {
		return Result{Status: StatusFailed, ErrorCode: "upstream", ErrorMessage: "provider rejected request"}, nil
	}})
	result, err := runtime.Execute(context.Background(), Request{ScopeID: "scope-1", Task: "inspect", EventSink: func(event Event) error {
		events = append(events, event)
		return nil
	}})
	if err != nil || result.Status != StatusFailed {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(events) != 2 || events[1].Type != "execution.failed" || events[1].Status != StatusFailed {
		t.Fatalf("unexpected failed lifecycle events: %+v", events)
	}
	if events[1].Payload["error_code"] != "upstream" {
		t.Fatalf("failed event did not preserve error code: %+v", events[1])
	}
}

func TestRuntimeStreamRejectsUnavailableRunner(t *testing.T) {
	if _, err := New(nil).Stream(context.Background(), Request{ScopeID: "scope-1", Task: "inspect"}); !errors.Is(err, ErrRunnerUnavailable) {
		t.Fatalf("expected ErrRunnerUnavailable, got %v", err)
	}
}

type fakeRuntimeContextResolver struct {
	resolved ResolvedContext
	err      error
}

type fakeEventStore struct{ events []Event }

func (s *fakeEventStore) AppendEvent(_ context.Context, event Event) error {
	s.events = append(s.events, event)
	return nil
}
func (s *fakeEventStore) ListEvents(_ context.Context, _ string, _ int64, _ int) ([]Event, error) {
	return s.events, nil
}

func TestRuntimePersistsLifecycleEvents(t *testing.T) {
	store := &fakeEventStore{}
	runtime := NewWithContextAndStore(fakeRunner{run: func(context.Context, Request) (Result, error) {
		return Result{Output: "ok"}, nil
	}}, nil, nil, store)
	if _, err := runtime.Execute(context.Background(), Request{ExecutionID: "exec-store", ScopeID: "scope-1", Task: "inspect"}); err != nil {
		t.Fatal(err)
	}
	if len(store.events) != 2 || store.events[0].Type != "execution.started" || store.events[1].Type != "execution.completed" {
		t.Fatalf("persisted events=%+v", store.events)
	}
}

func (f fakeRuntimeContextResolver) Resolve(context.Context, ContextRequest) (ResolvedContext, error) {
	return f.resolved, f.err
}

func TestRuntimeLoadsContextBeforeRunner(t *testing.T) {
	resolver := fakeRuntimeContextResolver{resolved: ResolvedContext{Resources: []ContextResource{{ID: "resource-1"}}, Tools: []ToolDefinition{{Name: "read", ResourceID: "resource-1"}}, Facts: []ContextFact{{ResourceID: "resource-1"}}}}
	runtime := NewWithContext(fakeRunner{run: func(_ context.Context, request Request) (Result, error) {
		if request.ResolvedContext == nil || len(request.ResolvedContext.Tools) != 1 || request.ToolGateway == nil {
			t.Fatalf("runner did not receive resolved context: %+v", request)
		}
		return Result{}, nil
	}}, resolver, NewPolicyGateway(NewToolRegistry(), nil, 0, 0, 0))
	var events []Event
	if _, err := runtime.Execute(context.Background(), Request{ScopeID: "scope-1", Task: "inspect", Context: ContextRequest{ResourceIDs: []string{"resource-1"}}, EventSink: func(event Event) error {
		events = append(events, event)
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[1].Type != "context.loaded" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestRuntimeResolvesAgentProfileBeforeRunner(t *testing.T) {
	profile := AgentProfile{ResourceID: "profile-1", ScopeID: "scope-1", Name: "diagnostic expert", Version: 3, Instruction: "Use evidence.", Capabilities: []string{"tool_calling"}, Enabled: true}
	var events []Event
	runtime := New(fakeRunner{run: func(_ context.Context, request Request) (Result, error) {
		if request.ResolvedAgentProfile == nil || request.ResolvedAgentProfile.ResourceID != profile.ResourceID || request.ResolvedAgentProfile.Version != 3 {
			t.Fatalf("runner did not receive resolved AgentProfile: %+v", request.ResolvedAgentProfile)
		}
		return Result{Output: "ok"}, nil
	}}).WithAgentProfileResolver(fakeAgentProfileResolver{profile: profile})
	if _, err := runtime.Execute(context.Background(), Request{ScopeID: "scope-1", AgentProfileID: "profile-1", Task: "diagnose", EventSink: func(event Event) error {
		events = append(events, event)
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[1].Type != "agent_profile.resolved" || events[2].Type != "execution.completed" {
		t.Fatalf("unexpected profile lifecycle events: %+v", events)
	}
}

type fakeAgentProfileResolver struct{ profile AgentProfile }

func (f fakeAgentProfileResolver) Resolve(context.Context, string, string) (AgentProfile, error) {
	return f.profile, nil
}

func TestRuntimeFailsWhenContextResolutionFails(t *testing.T) {
	resolver := fakeRuntimeContextResolver{err: errors.New("resource is forbidden")}
	runtime := NewWithContext(fakeRunner{run: func(context.Context, Request) (Result, error) {
		t.Fatal("runner should not run after context resolution failure")
		return Result{}, nil
	}}, resolver, nil)
	result, err := runtime.Execute(context.Background(), Request{ScopeID: "scope-1", Task: "inspect", Context: ContextRequest{ResourceIDs: []string{"resource-1"}}})
	if err == nil || result.Status != StatusFailed || result.ErrorCode != "context_resolution" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestRuntimeCancelPropagatesContext(t *testing.T) {
	started := make(chan struct{})
	runtime := New(fakeRunner{run: func(ctx context.Context, _ Request) (Result, error) {
		close(started)
		<-ctx.Done()
		return Result{}, ctx.Err()
	}})
	done := make(chan error, 1)
	go func() {
		_, err := runtime.Execute(context.Background(), Request{ExecutionID: "exec-cancel", ScopeID: "scope-1", Task: "inspect"})
		done <- err
	}()
	<-started
	if err := runtime.Cancel(context.Background(), "exec-cancel"); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestRuntimeCancelEmitsCancelledLifecycleEvent(t *testing.T) {
	started := make(chan struct{})
	var events []Event
	runtime := New(fakeRunner{run: func(ctx context.Context, _ Request) (Result, error) {
		close(started)
		<-ctx.Done()
		return Result{}, ctx.Err()
	}})
	done := make(chan error, 1)
	go func() {
		_, err := runtime.Execute(context.Background(), Request{
			ExecutionID: "exec-cancel-event", ScopeID: "scope-1", Task: "inspect",
			EventSink: func(event Event) error {
				events = append(events, event)
				return nil
			},
		})
		done <- err
	}()
	<-started
	if err := runtime.Cancel(context.Background(), "exec-cancel-event"); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if len(events) != 2 || events[1].Type != "execution.cancelled" || events[1].Status != StatusCancelled {
		t.Fatalf("unexpected cancellation events: %+v", events)
	}
}

func TestRuntimeRejectsDuplicateExecutionID(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	runtime := New(fakeRunner{run: func(ctx context.Context, _ Request) (Result, error) {
		close(started)
		select {
		case <-release:
			return Result{}, nil
		case <-ctx.Done():
			return Result{}, ctx.Err()
		}
	}})
	firstDone := make(chan error, 1)
	go func() {
		_, err := runtime.Execute(context.Background(), Request{ExecutionID: "exec-duplicate", ScopeID: "scope-1", Task: "inspect"})
		firstDone <- err
	}()
	<-started
	if _, err := runtime.Execute(context.Background(), Request{ExecutionID: "exec-duplicate", ScopeID: "scope-1", Task: "inspect"}); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("expected ErrAlreadyRunning, got %v", err)
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first execution failed: %v", err)
	}
}

func TestRuntimeTimeoutReturnsCancelled(t *testing.T) {
	runtime := New(fakeRunner{run: func(ctx context.Context, _ Request) (Result, error) {
		<-ctx.Done()
		return Result{}, ctx.Err()
	}})
	result, err := runtime.Execute(context.Background(), Request{ScopeID: "scope-1", Task: "inspect", Budget: Budget{Timeout: time.Millisecond}})
	if !errors.Is(err, context.DeadlineExceeded) || result.Status != StatusCancelled || result.ErrorCode != "timeout" {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
}

func TestRuntimeStreamReturnsEvents(t *testing.T) {
	runtime := New(fakeRunner{run: func(_ context.Context, request Request) (Result, error) {
		_ = request.EventSink(Event{Type: "assistant.delta", Payload: map[string]any{"text": "ok"}})
		return Result{Output: "ok"}, nil
	}})
	stream, err := runtime.Stream(context.Background(), Request{ScopeID: "scope-1", Task: "inspect"})
	if err != nil {
		t.Fatal(err)
	}
	var got []Event
	for event := range stream {
		got = append(got, event)
	}
	if len(got) != 3 || got[1].Type != "assistant.delta" || got[2].Type != "execution.completed" {
		t.Fatalf("unexpected stream: %+v", got)
	}
}
