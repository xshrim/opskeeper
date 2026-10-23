package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Runtime struct {
	runner          Runner
	resolver        ContextResolver
	personaResolver PersonaResolver
	plans           PlanResolver
	gateway         *PolicyGateway
	store           EventStore
	active          sync.Map
}

type activeExecution struct{ cancel context.CancelFunc }

func New(runner Runner) *Runtime { return &Runtime{runner: runner} }

func NewWithContext(runner Runner, resolver ContextResolver, gateway *PolicyGateway) *Runtime {
	return &Runtime{runner: runner, resolver: resolver, gateway: gateway}
}

func NewWithContextAndStore(runner Runner, resolver ContextResolver, gateway *PolicyGateway, store EventStore) *Runtime {
	return &Runtime{runner: runner, resolver: resolver, gateway: gateway, store: store}
}

// WithPersonaResolver attaches the independent expert resolver
// resolver without changing the existing constructor signatures.
func (r *Runtime) WithPersonaResolver(resolver PersonaResolver) *Runtime {
	r.personaResolver = resolver
	return r
}

// WithPlanResolver attaches a side-effect-free source of execution plans.
// The resolver supplies prompt/tool/schema inputs; Engine still performs
// the only model and tool execution.
func (r *Runtime) WithPlanResolver(resolver PlanResolver) *Runtime {
	r.plans = resolver
	return r
}

func (r *Runtime) Name() string { return "Engine" }

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
	executionStartedAt := time.Now()
	var sinkMu sync.Mutex
	var sinkErrMu sync.Mutex
	var sinkErr error
	recordSinkErr := func(err error) {
		if err == nil {
			return
		}
		sinkErrMu.Lock()
		if sinkErr == nil {
			sinkErr = err
		}
		sinkErrMu.Unlock()
	}
	originalSink := request.EventSink
	sink := func(event Event) error {
		sinkMu.Lock()
		defer sinkMu.Unlock()
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
		if event.Payload == nil {
			event.Payload = make(map[string]any)
		}
		// Runtime owns the execution clock. Always replace a nested runner's
		// relative value so persisted/SSE events share the execution.started base.
		event.Payload["elapsed_ms"] = time.Since(executionStartedAt).Milliseconds()
		var persistenceErr error
		if r.store != nil {
			if err := r.store.AppendEvent(context.Background(), event); err != nil {
				persistenceErr = fmt.Errorf("persist Engine event: %w", err)
				recordSinkErr(persistenceErr)
			}
		}
		if originalSink == nil {
			return persistenceErr
		}
		if err := originalSink(event); err != nil {
			recordSinkErr(err)
			return err
		}
		return persistenceErr
	}
	request.EventSink = sink
	// Keep the gateway available to the runner even when no context resources
	// were resolved. Besides normal tool dispatch, it owns the optional audit
	// sink used to record unknown or argument-invalid tool attempts.
	if r.gateway != nil {
		request.ToolGateway = r.gateway
	}
	if err := sink(Event{Type: "execution.started", Status: StatusRunning, Payload: map[string]any{"profile": request.Profile}}); err != nil {
		// Do not execute model or tool side effects when the durable event path
		// is already unavailable. A run without an auditable start cannot be
		// reported as successful or safely resumed.
		result := Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "event_sink", ErrorMessage: publicError(err)}
		_ = sink(Event{Type: "execution.failed", Status: StatusFailed, Payload: map[string]any{"error_code": result.ErrorCode, "error": result.ErrorMessage}})
		return result, err
	}
	if request.SkillID != "" || request.SkillVersionID != "" {
		if r.plans == nil {
			return failedRuntimeResult(request, sink, "plan_unavailable", "execution plan resolver is unavailable")
		}
		plan, planErr := r.plans.ResolvePlan(ctx, request.ScopeID, request.SkillID, request.SkillVersionID)
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
		_ = sink(Event{Type: "execution.plan.resolved", Status: StatusRunning, Payload: map[string]any{"source_skill_id": plan.SourceSkillID, "source_version_id": plan.SourceVersionID, "name": plan.Name, "tool_count": len(plan.Tools)}})
	}
	executionBaseInstruction := request.Instruction
	if request.PersonaID != "" {
		if r.personaResolver == nil {
			result := Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "persona_unavailable", ErrorMessage: "persona resolver is unavailable"}
			_ = sink(Event{Type: "execution.failed", Status: StatusFailed, Payload: map[string]any{"error_code": result.ErrorCode, "error": result.ErrorMessage}})
			return result, errors.New(result.ErrorMessage)
		}
		profile, profileErr := r.personaResolver.Resolve(ctx, request.ScopeID, request.PersonaID)
		if profileErr != nil {
			message := publicError(profileErr)
			result := Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "persona_resolution", ErrorMessage: message}
			_ = sink(Event{Type: "execution.failed", Status: StatusFailed, Payload: map[string]any{"error_code": result.ErrorCode, "error": result.ErrorMessage}})
			return result, profileErr
		}
		request.ResolvedPersona = &profile
		if profile.Instruction != "" && request.Instruction != "" {
			request.Instruction = profile.Instruction + "\n\n" + request.Instruction
		} else if profile.Instruction != "" {
			request.Instruction = profile.Instruction
		}
		_ = sink(Event{Type: "persona.resolved", Status: StatusRunning, Payload: map[string]any{"persona_id": profile.PersonaID, "name": profile.Name, "version": profile.Version, "capabilities": profile.Capabilities, "allowed_tool_count": len(profile.AllowedTools)}})
	}
	if r.resolver != nil && len(request.Context.ResourceIDs) > 0 {
		if request.ResolvedPersona != nil && request.ResolvedPersona.Role == "worker" {
			request.Context.RestrictKinds = true
			if strings.TrimSpace(request.ResolvedPersona.Domain) != "" {
				request.Context.AllowedKinds = []string{request.ResolvedPersona.Domain}
			}
		}
		if request.ResolvedPersona != nil && request.ResolvedPersona.Role == "manager" {
			request.Context.MetadataOnly = true
		}
		resolved, resolveErr := r.resolver.Resolve(ctx, request.Context)
		if resolveErr != nil {
			message := publicError(resolveErr)
			result := Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "context_resolution", ErrorMessage: message}
			_ = sink(Event{Type: "execution.failed", Status: StatusFailed, Payload: map[string]any{"error_code": result.ErrorCode, "error": result.ErrorMessage}})
			return result, resolveErr
		}
		request.ResolvedContext = &resolved
		if r.gateway != nil {
			request.ToolGateway = r.gateway
		}
		if request.ResolvedPersona != nil && request.ResolvedPersona.Role == "worker" {
			request.ResolvedContext = filterPersonaDomain(resolved, request.ResolvedPersona.Domain)
			filteredResourceIDs := make([]string, 0, len(request.ResolvedContext.Resources))
			for _, resource := range request.ResolvedContext.Resources {
				filteredResourceIDs = append(filteredResourceIDs, resource.ID)
			}
			request.Context.ResourceIDs = filteredResourceIDs
			if request.ResolvedPersona.Domain != "" {
				request.Instruction += "\n\n资源领域约束：你是 " + request.ResolvedPersona.Domain + " 资源专家，只能对本次传入的该领域资源进行诊断；不得推断或声称检查了其他领域资源。"
			}
			if len(request.ResolvedContext.Tools) > 0 {
				request.Requirements.Capabilities = append(request.Requirements.Capabilities, "tool_calling")
			}
		}
		_ = sink(Event{Type: "context.loaded", Status: StatusRunning, Payload: map[string]any{"resource_count": len(request.ResolvedContext.Resources), "tool_count": len(request.ResolvedContext.Tools), "fact_count": len(request.ResolvedContext.Facts)}})
	}
	if request.ResolvedPersona != nil && request.ResolvedPersona.Role == "manager" && request.DelegationDepth == 0 {
		delegationTools, delegationErr := r.personaDelegationTools(ctx, request, executionBaseInstruction, sink)
		if delegationErr != nil {
			return failedRuntimeError(request, sink, "persona_delegation", delegationErr)
		}
		request.InternalTools = delegationTools
		if len(delegationTools) > 0 {
			request.Requirements.Capabilities = append(request.Requirements.Capabilities, "tool_calling")
		}
		request.Context.ResourceIDs = nil
		request.ResolvedContext = &ResolvedContext{}
		request.Input = maps.Clone(request.Input)
		delete(request.Input, "target_resource_ids")
		request.Instruction += "\n\n职能专家约束：负责拆解诊断问题，并按需调用已提供的资源专家委派工具。只基于资源专家返回的报告汇总结论，明确指出未能检查或证据不足的部分；不得直接声称检查了资源，也不得臆造检查结果。"
		// Managers reason over worker reports; they do not receive selected resource IDs or raw context snapshots.
	}

	var result Result
	var err error
	if streaming, ok := r.runner.(StreamingRunner); ok && request.Stream {
		result, err = streaming.RunStream(ctx, request, sink)
	} else {
		result, err = r.runner.Run(ctx, request)
	}
	if err != nil {
		sinkErrMu.Lock()
		eventErr := sinkErr
		sinkErrMu.Unlock()
		if eventErr != nil {
			result.ExecutionID, result.Status, result.ErrorCode = request.ExecutionID, StatusFailed, "event_sink"
			result.ErrorMessage = publicError(eventErr)
			_ = sink(Event{Type: "execution.failed", Status: StatusFailed, Payload: map[string]any{"error_code": result.ErrorCode, "error": result.ErrorMessage}})
			return result, eventErr
		}
		status, code := result.Status, result.ErrorCode
		if status != StatusCancelled && status != StatusFailed {
			status = StatusFailed
		}
		if code == "" {
			code = "runtime"
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			status, code = StatusCancelled, "timeout"
		} else if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
			status, code = StatusCancelled, "cancelled"
		}
		result.ExecutionID, result.Status, result.ErrorCode = request.ExecutionID, status, code
		if strings.TrimSpace(result.ErrorMessage) == "" {
			result.ErrorMessage = publicError(err)
		} else {
			result.ErrorMessage = publicError(errors.New(result.ErrorMessage))
		}
		eventType := "execution.failed"
		if status == StatusCancelled {
			eventType = "execution.cancelled"
		}
		_ = sink(Event{Type: eventType, Status: status, Payload: map[string]any{"error_code": code, "error": result.ErrorMessage}})
		return result, err
	}
	sinkErrMu.Lock()
	eventErr := sinkErr
	sinkErrMu.Unlock()
	if eventErr != nil {
		result.ExecutionID, result.Status, result.ErrorCode = request.ExecutionID, StatusFailed, "event_sink"
		result.ErrorMessage = publicError(eventErr)
		_ = sink(Event{Type: "execution.failed", Status: StatusFailed, Payload: map[string]any{"error_code": result.ErrorCode, "error": result.ErrorMessage}})
		return result, eventErr
	}
	if err := ctx.Err(); err != nil {
		result.ExecutionID, result.Status, result.ErrorCode, result.ErrorMessage = request.ExecutionID, StatusCancelled, "cancelled", publicError(err)
		_ = sink(Event{Type: "execution.cancelled", Status: StatusCancelled, Payload: map[string]any{"error": publicError(err)}})
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

func filterPersonaDomain(resolved ResolvedContext, domain string) *ResolvedContext {
	filtered := &ResolvedContext{}
	resourceIDs := make(map[string]struct{})
	for _, resource := range resolved.Resources {
		if personaDomainMatches(domain, resource.Kind) {
			filtered.Resources = append(filtered.Resources, resource)
			resourceIDs[resource.ID] = struct{}{}
		}
	}
	for _, tool := range resolved.Tools {
		if _, ok := resourceIDs[tool.ResourceID]; ok {
			filtered.Tools = append(filtered.Tools, tool)
		}
	}
	for _, fact := range resolved.Facts {
		if _, ok := resourceIDs[fact.ResourceID]; ok {
			filtered.Facts = append(filtered.Facts, fact)
		}
	}
	return filtered
}

func (r *Runtime) personaDelegationTools(parent context.Context, request Request, baseInstruction string, sink EventSink) ([]Tool, error) {
	manager := request.ResolvedPersona
	if manager == nil || r.personaResolver == nil || request.ResolvedContext == nil {
		return nil, nil
	}
	limit := manager.MaxDispatch
	if limit < 1 || limit > 5 {
		return nil, fmt.Errorf("manager Persona has invalid dispatch limit")
	}
	resources := append([]ContextResource(nil), request.ResolvedContext.Resources...)
	usedNames := make(map[string]struct{})
	for _, binding := range buildToolBindings(request.ResolvedContext.Tools) {
		usedNames[binding.modelName] = struct{}{}
	}
	var dispatched atomic.Int64
	tools := make([]Tool, 0, len(manager.WorkerPersonaIDs))
	for index, workerID := range manager.WorkerPersonaIDs {
		worker, err := r.personaResolver.Resolve(parent, request.ScopeID, workerID)
		if err != nil {
			return nil, fmt.Errorf("resolve worker Persona %s: %w", workerID, err)
		}
		if worker.Role != "worker" {
			return nil, fmt.Errorf("delegated Persona %s must be a domain worker", workerID)
		}
		if worker.ScopeID != request.ScopeID {
			return nil, fmt.Errorf("delegated Persona %s must belong to the current scope", workerID)
		}
		if strings.TrimSpace(worker.Domain) == "" {
			return nil, fmt.Errorf("delegated Persona %s has no resource domain", workerID)
		}
		resourceIDs := make([]string, 0)
		resourceNames := make([]string, 0)
		for _, resource := range resources {
			if personaDomainMatches(worker.Domain, resource.Kind) {
				resourceIDs = append(resourceIDs, resource.ID)
				resourceNames = append(resourceNames, resource.Name)
			}
		}
		if len(resourceIDs) == 0 {
			_ = sink(Event{ExecutionID: request.ExecutionID, Type: "persona.delegation.skipped", Status: StatusRunning, Payload: map[string]any{"persona_id": worker.PersonaID, "reason": "no selected resources match worker domain"}})
			continue
		}
		name := uniqueModelToolName(fmt.Sprintf("delegate_%d_%s", index+1, sanitizeToolNamePart(worker.PersonaID)), usedNames)
		usedNames[name] = struct{}{}
		workerProfile := worker
		workerResourceIDs := append([]string(nil), resourceIDs...)
		workerResourceNames := append([]string(nil), resourceNames...)
		def := ToolDefinition{
			Name: name, Source: "persona-delegation", ReadOnly: true,
			Description: fmt.Sprintf("委派给%s（%s领域）检查已选且已授权资源：%s。传入具体诊断问题。", worker.Name, worker.Domain, strings.Join(resourceNames, "、")),
			InputSchema: json.RawMessage(`{"type":"object","required":["question"],"properties":{"question":{"type":"string","minLength":1,"maxLength":4000}},"additionalProperties":false}`),
		}
		tools = append(tools, ToolFunc{Def: def, Fn: func(toolCtx context.Context, args map[string]any) (ToolResult, error) {
			if dispatched.Add(1) > int64(limit) {
				return ToolResult{Output: map[string]any{"status": "rejected", "reason": "manager dispatch limit reached"}}, nil
			}
			question, _ := args["question"].(string)
			question = strings.TrimSpace(question)
			if question == "" {
				return ToolResult{Output: map[string]any{"status": "rejected", "reason": "question is required"}}, nil
			}
			childID := request.ExecutionID + ":worker:" + sanitizeToolNamePart(workerProfile.PersonaID) + ":" + fmt.Sprint(dispatched.Load())
			if err := sink(Event{ExecutionID: request.ExecutionID, Type: "persona.delegation.started", Status: StatusRunning, Payload: map[string]any{"persona_id": workerProfile.PersonaID, "persona_name": workerProfile.Name, "domain": workerProfile.Domain, "resource_ids": workerResourceIDs, "child_execution_id": childID}}); err != nil {
				return ToolResult{}, err
			}
			childBudget := request.Budget
			if childBudget.MaxIterations > 20 {
				childBudget.MaxIterations = 20
			}
			if childBudget.MaxToolCalls > 20 {
				childBudget.MaxToolCalls = 20
			}
			if childBudget.MaxTokens > 100000 {
				childBudget.MaxTokens = 100000
			}
			if childBudget.Timeout > 5*time.Minute {
				childBudget.Timeout = 5 * time.Minute
			}
			if childBudget.MaxOutputTokens > 8192 {
				childBudget.MaxOutputTokens = 8192
			}
			if childBudget.MaxOutputBytes > 64<<10 {
				childBudget.MaxOutputBytes = 64 << 10
			}
			workerInput := maps.Clone(request.Input)
			if workerInput == nil {
				workerInput = make(map[string]any)
			}
			workerInput["question"] = question
			workerInput["target_resource_ids"] = workerResourceIDs
			result, runErr := r.Execute(toolCtx, Request{
				ExecutionID: childID, ActorID: request.ActorID, ScopeID: request.ScopeID,
				Purpose: request.Purpose, ProviderID: request.ProviderID, ModelName: request.ModelName,
				Profile: request.Profile, PersonaID: workerProfile.PersonaID, Task: question, Instruction: baseInstruction, Input: workerInput,
				Context: ContextRequest{ResourceIDs: workerResourceIDs},
				SkillID: request.SkillID, Budget: childBudget, DelegationDepth: 1,
			})
			status := result.Status
			if runErr != nil && status != StatusCancelled {
				status = StatusFailed
			} else if status == "" {
				status = StatusFailed
			}
			output := map[string]any{"persona_id": workerProfile.PersonaID, "persona_name": workerProfile.Name, "domain": workerProfile.Domain, "resource_names": workerResourceNames, "status": string(status), "report": result.Output}
			if runErr != nil {
				output["error"] = publicError(runErr)
			} else if result.Status != StatusSucceeded && strings.TrimSpace(result.ErrorMessage) != "" {
				output["error"] = result.ErrorMessage
			}
			eventType := "persona.delegation.completed"
			if status != StatusSucceeded {
				eventType = "persona.delegation.failed"
			}
			_ = sink(Event{ExecutionID: request.ExecutionID, Type: eventType, Status: status, Payload: map[string]any{"persona_id": workerProfile.PersonaID, "status": status, "resource_ids": workerResourceIDs, "child_execution_id": childID}})
			return ToolResult{Output: output, Untrusted: true}, nil
		}})
	}
	return tools, nil
}

func personaDomainMatches(domain, kind string) bool {
	normalize := func(value string) string {
		value = strings.ToLower(strings.TrimSpace(value))
		switch value {
		case "postgres", "postgresql":
			return "postgresql"
		case "k8s", "kubernetes":
			return "kubernetes"
		default:
			return value
		}
	}
	return normalize(domain) == normalize(kind)
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
	message := publicError(err)
	result := Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: code, ErrorMessage: message}
	_ = sink(Event{Type: "execution.failed", Status: StatusFailed, Payload: map[string]any{"error_code": code, "error": message}})
	return result, err
}
