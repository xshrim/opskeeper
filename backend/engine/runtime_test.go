package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	run func(context.Context, Request) (Result, error)
}

type fakePlanResolver struct{ calls int }

func (f *fakePlanResolver) ResolvePlan(context.Context, string, string, string) (ExecutionPlan, error) {
	f.calls++
	return ExecutionPlan{SourceSkillID: "skill-1", SourceVersionID: "version-1", Instruction: "plan instruction"}, nil
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
	result, err := runtime.Execute(context.Background(), Request{ScopeID: "scope-1", SkillID: "skill-1", Task: "inspect"})
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
	if request.Profile != ProfileInteractive || request.Budget.MaxToolCalls != 100 || request.Budget.MaxTokens != 1000000 || request.Budget.MaxOutputTokens != 128000 || request.Budget.Timeout != 30*time.Minute {
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
	for _, event := range events {
		elapsed, ok := event.Payload["elapsed_ms"].(int64)
		if !ok || elapsed < 0 {
			t.Fatalf("event is missing server elapsed time: %+v", event)
		}
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

func TestRuntimePreservesRunnerErrorCode(t *testing.T) {
	runtime := New(fakeRunner{run: func(context.Context, Request) (Result, error) {
		return Result{Status: StatusFailed, ErrorCode: "empty_output", ErrorMessage: "模型未返回可展示的最终回答，请重试。"}, errors.New("model returned an empty final response")
	}})
	result, err := runtime.Execute(context.Background(), Request{ScopeID: "scope-1", Task: "inspect"})
	if err == nil || result.Status != StatusFailed || result.ErrorCode != "empty_output" {
		t.Fatalf("result=%+v err=%v, want runner error code preserved", result, err)
	}
}

func TestRuntimePreservesRunnerErrorMessageInTerminalEvent(t *testing.T) {
	var events []Event
	runtime := New(fakeRunner{run: func(context.Context, Request) (Result, error) {
		return Result{Status: StatusFailed, ErrorCode: "empty_output", ErrorMessage: "安全错误消息"}, errors.New("internal provider wording")
	}})
	result, err := runtime.Execute(context.Background(), Request{ScopeID: "scope-1", Task: "inspect", EventSink: func(event Event) error {
		events = append(events, event)
		return nil
	}})
	if err == nil || result.ErrorMessage != "安全错误消息" {
		t.Fatalf("result=%+v err=%v, want runner message preserved", result, err)
	}
	if len(events) != 2 || events[1].Payload["error"] != "安全错误消息" {
		t.Fatalf("events=%+v, want safe terminal error message", events)
	}
}

func TestRuntimeNormalizesSuccessfulStatusWhenRunnerReturnsError(t *testing.T) {
	var events []Event
	runtime := New(fakeRunner{run: func(context.Context, Request) (Result, error) {
		return Result{Status: StatusSucceeded, ErrorCode: "provider", ErrorMessage: "provider failed"}, errors.New("provider failed")
	}})
	result, err := runtime.Execute(context.Background(), Request{ScopeID: "scope-1", Task: "inspect", EventSink: func(event Event) error {
		events = append(events, event)
		return nil
	}})
	if err == nil || result.Status != StatusFailed || result.ErrorCode != "provider" {
		t.Fatalf("result=%+v err=%v, want failed status with preserved code", result, err)
	}
	if len(events) != 2 || events[1].Type != "execution.failed" || events[1].Status != StatusFailed {
		t.Fatalf("events=%+v, want failed terminal event", events)
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
	request  *ContextRequest
}

type fakeEventStore struct{ events []Event }

func (s *fakeEventStore) AppendEvent(_ context.Context, event Event) error {
	s.events = append(s.events, event)
	return nil
}
func (s *fakeEventStore) ListEvents(_ context.Context, _ string, _ int64, _ int) ([]Event, error) {
	return s.events, nil
}

type failingEventStore struct{ err error }

func (s failingEventStore) AppendEvent(context.Context, Event) error { return s.err }
func (s failingEventStore) ListEvents(context.Context, string, int64, int) ([]Event, error) {
	return nil, s.err
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

func TestRuntimeDoesNotCompleteWhenEventPersistenceFails(t *testing.T) {
	storeErr := errors.New("event database unavailable")
	runtime := NewWithContextAndStore(fakeRunner{run: func(context.Context, Request) (Result, error) {
		return Result{Output: "should not be accepted as durable"}, nil
	}}, nil, nil, failingEventStore{err: storeErr})
	result, err := runtime.Execute(context.Background(), Request{ExecutionID: "exec-store-failure", ScopeID: "scope-1", Task: "inspect"})
	if err == nil || result.Status != StatusFailed || result.ErrorCode != "event_sink" {
		t.Fatalf("result=%+v err=%v, want event persistence failure", result, err)
	}
}

func (f fakeRuntimeContextResolver) Resolve(_ context.Context, request ContextRequest) (ResolvedContext, error) {
	if f.request != nil {
		*f.request = request
	}
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

func TestRuntimePreservesRequestToolGatewayWhenRuntimeHasNone(t *testing.T) {
	gateway := NewPolicyGateway(NewToolRegistry(), nil, 0, 0, 0)
	var received Request
	runtime := New(fakeRunner{run: func(_ context.Context, request Request) (Result, error) {
		received = request
		return Result{Output: "ok"}, nil
	}})
	if _, err := runtime.Execute(context.Background(), Request{
		ScopeID: "scope-1", Task: "inspect", ToolGateway: gateway,
	}); err != nil {
		t.Fatal(err)
	}
	if received.ToolGateway != gateway {
		t.Fatalf("runtime replaced caller-provided gateway: got %p, want %p", received.ToolGateway, gateway)
	}
}

func TestRuntimeResolvesPersonaBeforeRunner(t *testing.T) {
	profile := Persona{PersonaID: "profile-1", ScopeID: "scope-1", Name: "diagnostic expert", Version: 3, Instruction: "Use evidence.", Capabilities: []string{"tool_calling"}, Enabled: true}
	var events []Event
	runtime := New(fakeRunner{run: func(_ context.Context, request Request) (Result, error) {
		if request.ResolvedPersona == nil || request.ResolvedPersona.PersonaID != profile.PersonaID || request.ResolvedPersona.Version != 3 {
			t.Fatalf("runner did not receive resolved Persona: %+v", request.ResolvedPersona)
		}
		return Result{Output: "ok"}, nil
	}}).WithPersonaResolver(fakePersonaResolver{profile: profile})
	if _, err := runtime.Execute(context.Background(), Request{ScopeID: "scope-1", PersonaID: "profile-1", Task: "diagnose", EventSink: func(event Event) error {
		events = append(events, event)
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[1].Type != "persona.resolved" || events[2].Type != "execution.completed" {
		t.Fatalf("unexpected profile lifecycle events: %+v", events)
	}
}

func TestManagerDelegatesOnlyMatchingSelectedResourcesToWorker(t *testing.T) {
	manager := Persona{PersonaID: "manager", ScopeID: "scope-1", Name: "Operations Lead", Enabled: true, Role: "manager", WorkerPersonaIDs: []string{"postgres-worker"}, MaxDispatch: 2}
	worker := Persona{PersonaID: "postgres-worker", ScopeID: "scope-1", Name: "PostgreSQL Expert", Enabled: true, Role: "worker", Domain: "postgresql", Instruction: "Inspect PostgreSQL."}
	var lastContextRequest ContextRequest
	contextResolver := fakeRuntimeContextResolver{request: &lastContextRequest, resolved: ResolvedContext{Resources: []ContextResource{
		{ID: "db-1", ScopeID: "scope-1", Kind: "postgres", Name: "orders"},
		{ID: "cache-1", ScopeID: "scope-1", Kind: "redis", Name: "cache"},
	}, Tools: []ToolDefinition{
		{Name: "postgres_inspect", ResourceID: "db-1"},
		{Name: "redis_inspect", ResourceID: "cache-1"},
	}, Facts: []ContextFact{
		{ResourceID: "db-1", Kind: "postgres"},
		{ResourceID: "cache-1", Kind: "redis"},
	}}}
	var workerRequest Request
	runtime := NewWithContext(fakeRunner{run: func(ctx context.Context, request Request) (Result, error) {
		if request.PersonaID == worker.PersonaID {
			workerRequest = request
			return Result{Output: "database report", Status: StatusSucceeded}, nil
		}
		if len(request.Context.ResourceIDs) != 0 || len(request.ResolvedContext.Resources) != 0 || len(request.ResolvedContext.Tools) != 0 || !strings.Contains(request.Instruction, "职能专家约束") {
			t.Fatalf("manager received raw resources or missed its role contract: %+v", request)
		}
		if len(request.InternalTools) != 1 {
			t.Fatalf("manager got %d internal tools, want one matching worker", len(request.InternalTools))
		}
		output, err := request.InternalTools[0].Invoke(ctx, map[string]any{"question": "check database availability"})
		if err != nil {
			t.Fatalf("invoke delegation: %v", err)
		}
		if output.Output == nil {
			t.Fatal("delegation returned no worker report")
		}
		return Result{Output: "summary", Status: StatusSucceeded}, nil
	}}, contextResolver, nil).WithPersonaResolver(personaMapResolver{profiles: map[string]Persona{
		manager.PersonaID: manager,
		worker.PersonaID:  worker,
	}})
	_, err := runtime.Execute(context.Background(), Request{
		ExecutionID: "manager-exec", ScopeID: "scope-1", PersonaID: manager.PersonaID,
		Task: "diagnose selected resources", Context: ContextRequest{ResourceIDs: []string{"db-1", "cache-1"}},
	})
	if err != nil {
		t.Fatalf("manager execution: %v", err)
	}
	if workerRequest.PersonaID != worker.PersonaID || workerRequest.Task != "check database availability" {
		t.Fatalf("worker request did not use configured Persona/task: %+v", workerRequest)
	}
	if len(workerRequest.Context.ResourceIDs) != 1 || workerRequest.Context.ResourceIDs[0] != "db-1" {
		t.Fatalf("worker received resources outside its domain: %+v", workerRequest.Context.ResourceIDs)
	}
	if len(workerRequest.ResolvedContext.Resources) != 1 || len(workerRequest.ResolvedContext.Tools) != 1 || workerRequest.ResolvedContext.Tools[0].ResourceID != "db-1" || len(workerRequest.ResolvedContext.Facts) != 1 {
		t.Fatalf("worker received non-domain context: %+v", workerRequest.ResolvedContext)
	}
	if !lastContextRequest.RestrictKinds || len(lastContextRequest.AllowedKinds) != 1 || lastContextRequest.AllowedKinds[0] != "postgresql" {
		t.Fatalf("worker domain was not applied before resource providers: %+v", lastContextRequest)
	}
}

type fakePersonaResolver struct{ profile Persona }

func (f fakePersonaResolver) Resolve(context.Context, string, string) (Persona, error) {
	return f.profile, nil
}

type personaMapResolver struct{ profiles map[string]Persona }

func (r personaMapResolver) Resolve(_ context.Context, _ string, id string) (Persona, error) {
	profile, ok := r.profiles[id]
	if !ok {
		return Persona{}, errors.New("persona not found")
	}
	return profile, nil
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
