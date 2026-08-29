package aiengine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Runtime struct {
	runner        Runner
	resolver      ContextResolver
	agentProfiles AgentProfileResolver
	plans         PlanResolver
	gateway       *PolicyGateway
	store         EventStore
	active        sync.Map
}

type activeExecution struct{ cancel context.CancelFunc }

func New(runner Runner) *Runtime { return &Runtime{runner: runner} }

func NewWithContext(runner Runner, resolver ContextResolver, gateway *PolicyGateway) *Runtime {
	return &Runtime{runner: runner, resolver: resolver, gateway: gateway}
}

func NewWithContextAndStore(runner Runner, resolver ContextResolver, gateway *PolicyGateway, store EventStore) *Runtime {
	return &Runtime{runner: runner, resolver: resolver, gateway: gateway, store: store}
}

// WithAgentProfileResolver attaches the resource-backed expert profile
// resolver without changing the existing constructor signatures.
func (r *Runtime) WithAgentProfileResolver(resolver AgentProfileResolver) *Runtime {
	r.agentProfiles = resolver
	return r
}

// WithPlanResolver attaches a side-effect-free source of execution plans.
// The resolver supplies prompt/tool/schema inputs; AIEngine still performs
// the only model and tool execution.
func (r *Runtime) WithPlanResolver(resolver PlanResolver) *Runtime {
	r.plans = resolver
	return r
}

func (r *Runtime) Name() string { return "AIEngine" }

func (r *Runtime) Execute(parent context.Context, request Request) (Result, error) {
	if err := request.Normalize(); err != nil {
		return Result{}, err
	}
	if r.runner == nil {
		return Result{}, ErrRunnerUnavailable
	}
	if parent == nil {
		parent = context.Background()
	}
	if request.ExecutionID == "" {
		request.ExecutionID = fmt.Sprintf("exec-%d", time.Now().UnixNano())
	}
	ctx, cancel := context.WithTimeout(parent, request.Budget.Timeout)
	if _, loaded := r.active.LoadOrStore(request.ExecutionID, activeExecution{cancel: cancel}); loaded {
		cancel()
		return Result{}, ErrAlreadyRunning
	}
	defer func() {
		cancel()
		r.active.Delete(request.ExecutionID)
	}()

	var sequence atomic.Int64
	originalSink := request.EventSink
	sink := func(event Event) error {
		if event.ExecutionID == "" {
			event.ExecutionID = request.ExecutionID
		}
		if event.Timestamp.IsZero() {
			event.Timestamp = time.Now().UTC()
		}
		if event.Sequence <= 0 {
			event.Sequence = sequence.Add(1)
		} else {
			for {
				current := sequence.Load()
				next := event.Sequence
				if next <= current {
					next = current + 1
				}
				if sequence.CompareAndSwap(current, next) {
					event.Sequence = next
					break
				}
			}
		}
		if r.store != nil {
			_ = r.store.AppendEvent(context.Background(), event)
		}
		if originalSink == nil {
			return nil
		}
		return originalSink(event)
	}
	request.EventSink = sink
	_ = sink(Event{Type: "execution.started", Status: StatusRunning, Payload: map[string]any{"profile": request.Profile}})
	if request.SkillResourceID != "" || request.SkillVersionID != "" {
		if r.plans == nil {
			return failedRuntimeResult(request, sink, "plan_unavailable", "execution plan resolver is unavailable")
		}
		plan, planErr := r.plans.ResolvePlan(ctx, request.ScopeID, request.SkillResourceID, request.SkillVersionID)
		if planErr != nil {
			return failedRuntimeError(request, sink, "plan_resolution", planErr)
		}
		if strings.TrimSpace(plan.Instruction) == "" {
			return failedRuntimeResult(request, sink, "plan_invalid", "execution plan instruction is required")
		}
		request.Instruction = plan.Instruction
		request.InputSchema = plan.InputSchema
		request.OutputSchema = plan.OutputSchema
		request.AllowedTools = append([]string(nil), plan.AllowedTools...)
		request.RestrictTools = true
		request.Requirements.Capabilities = append(request.Requirements.Capabilities, plan.Capabilities...)
		if len(request.AllowedTools) == 0 && len(plan.Tools) > 0 {
			request.AllowedTools = make([]string, 0, len(plan.Tools))
			for _, declaration := range plan.Tools {
				request.AllowedTools = append(request.AllowedTools, declaration.Name)
			}
		}
		_ = sink(Event{Type: "execution.plan.resolved", Status: StatusRunning, Payload: map[string]any{"source_resource_id": plan.SourceResourceID, "source_version_id": plan.SourceVersionID, "name": plan.Name, "tool_count": len(plan.Tools)}})
	}
	if request.AgentProfileID != "" {
		if r.agentProfiles == nil {
			result := Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "agent_profile_unavailable", ErrorMessage: "agent profile resolver is unavailable"}
			_ = sink(Event{Type: "execution.failed", Status: StatusFailed, Payload: map[string]any{"error_code": result.ErrorCode, "error": result.ErrorMessage}})
			return result, errors.New(result.ErrorMessage)
		}
		profile, profileErr := r.agentProfiles.Resolve(ctx, request.ScopeID, request.AgentProfileID)
		if profileErr != nil {
			result := Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "agent_profile_resolution", ErrorMessage: profileErr.Error()}
			_ = sink(Event{Type: "execution.failed", Status: StatusFailed, Payload: map[string]any{"error_code": result.ErrorCode, "error": result.ErrorMessage}})
			return result, profileErr
		}
		request.ResolvedAgentProfile = &profile
		if profile.Instruction != "" && request.Instruction != "" {
			request.Instruction = profile.Instruction + "\n\n" + request.Instruction
		} else if profile.Instruction != "" {
			request.Instruction = profile.Instruction
		}
		_ = sink(Event{Type: "agent_profile.resolved", Status: StatusRunning, Payload: map[string]any{"resource_id": profile.ResourceID, "name": profile.Name, "version": profile.Version, "capabilities": profile.Capabilities, "allowed_tool_count": len(profile.AllowedTools)}})
	}
	if r.resolver != nil && len(request.Context.ResourceIDs) > 0 {
		resolved, resolveErr := r.resolver.Resolve(ctx, request.Context)
		if resolveErr != nil {
			result := Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "context_resolution", ErrorMessage: resolveErr.Error()}
			_ = sink(Event{Type: "execution.failed", Status: StatusFailed, Payload: map[string]any{"error_code": result.ErrorCode, "error": result.ErrorMessage}})
			return result, resolveErr
		}
		request.ResolvedContext = &resolved
		request.ToolGateway = r.gateway
		_ = sink(Event{Type: "context.loaded", Status: StatusRunning, Payload: map[string]any{"resource_count": len(resolved.Resources), "tool_count": len(resolved.Tools), "fact_count": len(resolved.Facts)}})
	}

	var result Result
	var err error
	if streaming, ok := r.runner.(StreamingRunner); ok && request.Stream {
		result, err = streaming.RunStream(ctx, request, sink)
	} else {
		result, err = r.runner.Run(ctx, request)
	}
	if err != nil {
		status, code := StatusFailed, "runtime"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			status, code = StatusCancelled, "timeout"
		} else if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
			status, code = StatusCancelled, "cancelled"
		}
		result.ExecutionID, result.Status, result.ErrorCode = request.ExecutionID, status, code
		result.ErrorMessage = err.Error()
		eventType := "execution.failed"
		if status == StatusCancelled {
			eventType = "execution.cancelled"
		}
		_ = sink(Event{Type: eventType, Status: status, Payload: map[string]any{"error_code": code, "error": err.Error()}})
		return result, err
	}
	if err := ctx.Err(); err != nil {
		result.ExecutionID, result.Status, result.ErrorCode, result.ErrorMessage = request.ExecutionID, StatusCancelled, "cancelled", err.Error()
		_ = sink(Event{Type: "execution.cancelled", Status: StatusCancelled, Payload: map[string]any{"error": err.Error()}})
		return result, err
	}
	result.ExecutionID = request.ExecutionID
	if result.Status == "" {
		result.Status = StatusSucceeded
	}
	if result.Status == StatusFailed {
		_ = sink(Event{Type: "execution.failed", Status: result.Status, Payload: map[string]any{
			"error_code": result.ErrorCode, "error": result.ErrorMessage,
			"tool_call_count": result.ToolCallCount, "total_tokens": result.TotalTokens,
		}})
	} else if result.Status == StatusCancelled {
		_ = sink(Event{Type: "execution.cancelled", Status: result.Status, Payload: map[string]any{
			"error_code": result.ErrorCode, "error": result.ErrorMessage,
			"tool_call_count": result.ToolCallCount, "total_tokens": result.TotalTokens,
		}})
	} else {
		_ = sink(Event{Type: "execution.completed", Status: result.Status, Payload: map[string]any{"tool_call_count": result.ToolCallCount, "total_tokens": result.TotalTokens}})
	}
	return result, nil
}

func (r *Runtime) Stream(ctx context.Context, request Request) (<-chan Event, error) {
	if err := request.Normalize(); err != nil {
		return nil, err
	}
	if r.runner == nil {
		return nil, ErrRunnerUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	events := make(chan Event, 32)
	original := request.EventSink
	request.Stream = true
	request.EventSink = func(event Event) error {
		if original != nil {
			if err := original(event); err != nil {
				return err
			}
		}
		select {
		case events <- event:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	go func() {
		defer close(events)
		_, _ = r.Execute(ctx, request)
	}()
	return events, nil
}

func (r *Runtime) Cancel(_ context.Context, executionID string) error {
	if value, ok := r.active.Load(executionID); ok {
		value.(activeExecution).cancel()
		return nil
	}
	return fmt.Errorf("execution %q is not running", executionID)
}

func failedRuntimeResult(request Request, sink EventSink, code, message string) (Result, error) {
	result := Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: code, ErrorMessage: message}
	_ = sink(Event{Type: "execution.failed", Status: StatusFailed, Payload: map[string]any{"error_code": code, "error": message}})
	return result, errors.New(message)
}

func failedRuntimeError(request Request, sink EventSink, code string, err error) (Result, error) {
	if err == nil {
		return failedRuntimeResult(request, sink, code, "execution plan resolution failed")
	}
	result := Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: code, ErrorMessage: err.Error()}
	_ = sink(Event{Type: "execution.failed", Status: StatusFailed, Payload: map[string]any{"error_code": code, "error": err.Error()}})
	return result, err
}
