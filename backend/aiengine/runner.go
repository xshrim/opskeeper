package aiengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	adkjsonschema "github.com/google/jsonschema-go/jsonschema"
	tekurischema "github.com/santhosh-tekuri/jsonschema/v6"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/genai"
)

type executionState struct {
	mu               sync.RWMutex
	observations     map[string]any
	confirmedFacts   []string
	hypotheses       []string
	eliminatedCauses []string
	openQuestions    []string
	nextActions      []string
}

func newExecutionState() *executionState { return &executionState{observations: make(map[string]any)} }

func (s *executionState) save(value any) string {
	id := fmt.Sprintf("obs-%d", time.Now().UnixNano())
	s.mu.Lock()
	s.observations[id] = value
	s.mu.Unlock()
	return id
}

func (s *executionState) read(id string, offset, limit int) (any, bool) {
	s.mu.RLock()
	value, ok := s.observations[id]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	encoded, _ := json.Marshal(value)
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 8000
	}
	if offset > len(encoded) {
		offset = len(encoded)
	}
	end := offset + limit
	if end > len(encoded) {
		end = len(encoded)
	}
	return map[string]any{"observation_id": id, "offset": offset, "next_offset": end, "done": end >= len(encoded), "content": string(encoded[offset:end])}, true
}

func (s *executionState) observe(toolName string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	text := safeProgressText(fmt.Sprint(value), 240)
	if text == "" {
		return
	}
	if strings.Contains(strings.ToLower(text), "error") || strings.Contains(strings.ToLower(text), "failed") {
		s.openQuestions = appendBounded(s.openQuestions, toolName+": 返回了错误，需要重新评估")
		s.eliminatedCauses = appendBounded(s.eliminatedCauses, toolName+": 当前调用路径不可用")
		return
	}
	s.confirmedFacts = appendBounded(s.confirmedFacts, toolName+": 已获得环境观察")
}

func (s *executionState) prompt() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	encode := func(items []string) string { b, _ := json.Marshal(items); return string(b) }
	return "Execution state (machine-maintained, do not expose as chain of thought): confirmed_facts=" + encode(s.confirmedFacts) + ", hypotheses=" + encode(s.hypotheses) + ", eliminated_causes=" + encode(s.eliminatedCauses) + ", open_questions=" + encode(s.openQuestions) + ", next_actions=" + encode(s.nextActions)
}

func appendBounded(items []string, value string) []string {
	if len(items) > 12 {
		items = items[len(items)-12:]
	}
	return append(items, value)
}

// AgentRunner is the Skill-independent Agent execution runtime. It owns the
// model call, ADK reasoning loop, generic context tools, budgets and output
// contract validation. Skill is represented only by optional fields on
// Request (instruction, schemas and tool declarations are supplied by the
// Skill adapter when present).
type AgentRunner struct {
	BuildModel ModelBuilder
	AppName    string
}

func NewAgentRunner(buildModel ModelBuilder) *AgentRunner {
	return &AgentRunner{BuildModel: buildModel, AppName: "opskeeper-aiengine"}
}

func (r *AgentRunner) Run(ctx context.Context, request Request) (Result, error) {
	return r.execute(ctx, request, nil)
}

func (r *AgentRunner) RunStream(ctx context.Context, request Request, sink EventSink) (Result, error) {
	// The streaming entry point is an explicit contract; callers should not
	// have to duplicate the flag in Request to receive provider deltas.
	request.Stream = true
	return r.execute(ctx, request, sink)
}

func (r *AgentRunner) execute(parent context.Context, request Request, sink EventSink) (Result, error) {
	if r == nil || r.BuildModel == nil {
		return Result{}, ErrRunnerUnavailable
	}
	if parent == nil {
		parent = context.Background()
	}
	// Keep the caller's value separate from the runtime default. A default
	// budget must never override the selected provider model's hard output cap.
	requestedMaxOutputTokens := request.Budget.MaxOutputTokens
	if request.Budget.Timeout <= 0 {
		request.Budget.Timeout = DefaultBudget().Timeout
	}
	if request.Budget.MaxIterations <= 0 {
		request.Budget.MaxIterations = DefaultBudget().MaxIterations
	}
	if request.Budget.MaxToolCalls <= 0 {
		request.Budget.MaxToolCalls = DefaultBudget().MaxToolCalls
	}
	if request.Budget.MaxTokens <= 0 {
		request.Budget.MaxTokens = DefaultBudget().MaxTokens
	}
	if request.Budget.MaxOutputTokens <= 0 {
		request.Budget.MaxOutputTokens = DefaultBudget().MaxOutputTokens
	}
	if request.Budget.MaxOutputBytes <= 0 {
		request.Budget.MaxOutputBytes = DefaultBudget().MaxOutputBytes
	}
	timeoutCtx, timeoutCancel := context.WithTimeout(parent, request.Budget.Timeout)
	defer timeoutCancel()
	ctx, loopCancel := context.WithCancel(timeoutCtx)
	defer loopCancel()
	if sink == nil {
		sink = request.EventSink
	}
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
		loopCancel()
	}
	getSinkErr := func() error {
		sinkErrMu.Lock()
		defer sinkErrMu.Unlock()
		return sinkErr
	}
	if sink != nil {
		rawSink := sink
		var eventMu sync.Mutex
		var eventSequence atomic.Int64
		var actionCount atomic.Int64
		executionStartedAt := time.Now()
		sink = func(event Event) error {
			eventMu.Lock()
			defer eventMu.Unlock()
			if event.ExecutionID == "" {
				event.ExecutionID = request.ExecutionID
			}
			if event.Timestamp.IsZero() {
				event.Timestamp = time.Now().UTC()
			}
			if event.Sequence <= 0 {
				event.Sequence = eventSequence.Add(1)
			} else {
				for {
					current := eventSequence.Load()
					next := event.Sequence
					if next <= current {
						next = current + 1
					}
					if eventSequence.CompareAndSwap(current, next) {
						event.Sequence = next
						break
					}
				}
			}
			if event.Payload == nil {
				event.Payload = make(map[string]any)
			}
			// Keep progress accounting on the server so streaming clients do not
			// have to infer action order or elapsed time from local timestamps.
			event.Payload["elapsed_ms"] = time.Since(executionStartedAt).Milliseconds()
			if event.Type == "tool.requested" {
				index := actionCount.Add(1)
				event.Payload["action_index"] = index
				event.Payload["action_count"] = index
			} else if strings.HasPrefix(event.Type, "tool.") {
				count := actionCount.Load()
				event.Payload["action_count"] = count
				if sequence, ok := event.Payload["call_sequence"].(int); ok && sequence > 0 {
					event.Payload["action_index"] = sequence
				} else if sequence, ok := event.Payload["call_sequence"].(int64); ok && sequence > 0 {
					event.Payload["action_index"] = sequence
				}
			}
			err := rawSink(event)
			if err != nil {
				recordSinkErr(err)
			}
			return err
		}
		request.EventSink = sink
	}
	if request.ExecutionID == "" {
		request.ExecutionID = fmt.Sprintf("exec-%d", time.Now().UnixNano())
	}
	modelResult, buildErr := r.BuildModel(ctx, request.ScopeID, request.AIProviderResourceID, request.ModelName, request.Purpose)
	if buildErr != nil {
		return Result{}, buildErr
	}
	if modelResult.Client == nil {
		return Result{}, errors.New("AIEngine model client is unavailable")
	}
	requiredCapabilities := append([]string(nil), request.Requirements.Capabilities...)
	if request.Stream || request.Requirements.Streaming {
		requiredCapabilities = append(requiredCapabilities, "stream")
	}
	if request.Requirements.StructuredOut {
		requiredCapabilities = append(requiredCapabilities, "structured_output")
	}
	if request.ResolvedAgentProfile != nil {
		requiredCapabilities = append(requiredCapabilities, request.ResolvedAgentProfile.Capabilities...)
	}
	if missing := missingCapabilities(modelResult.Capabilities, requiredCapabilities); len(missing) > 0 {
		return Result{}, fmt.Errorf("selected model lacks capabilities: %s", strings.Join(missing, ", "))
	}

	instruction := request.Instruction
	if strings.TrimSpace(instruction) == "" {
		if request.ResolvedAgentProfile != nil && strings.TrimSpace(request.ResolvedAgentProfile.Instruction) != "" {
			instruction = request.ResolvedAgentProfile.Instruction
		} else {
			instruction = "你是一个受控的 OpsKeeper AI 助手。只基于授权上下文回答问题，不执行未授权的写操作。"
		}
	}
	// Keep the model's interaction legible without asking for private chain of
	// thought. A short, user-safe progress sentence may precede each action;
	// tool results must be observed before planning the next dependent action.
	instruction = strings.TrimSpace(instruction) + `

	执行约束：采用逐轮的 ReAct 方式处理任务。每轮用一句简短、可展示的自然语言说明当前判断和下一步，不要使用“阶段 1”“阶段 2”或其他机械编号，也不要逐字重复上一轮摘要；只有出现新的观察或路径变化时才补充说明。随后只调用当前阶段必需的受控工具；同一轮中彼此独立且不依赖前序结果的工具可以并行调用，有依赖关系的工具应分到后续回合。等待本轮工具结果全部返回并汇总为 Observation 后，再决定下一步。工具返回错误时，明确说明错误原因和修正动作，不要把失败包装成已完成。不得臆造工具结果，不要输出隐藏思维链、凭据或敏感 Prompt。`
	prompt, err := executionPrompt(request)
	if err != nil {
		return Result{}, err
	}
	encodedInput, err := json.Marshal(request.Input)
	if err != nil {
		return Result{}, fmt.Errorf("AIEngine input must be JSON serializable: %w", err)
	}
	if len(request.InputSchema) > 0 {
		if err := validateJSON(request.InputSchema, encodedInput); err != nil {
			return Result{}, fmt.Errorf("validate execution plan input: %w", err)
		}
	} else if request.ResolvedAgentProfile != nil && len(request.ResolvedAgentProfile.InputSchema) > 0 {
		if err := validateJSON(request.ResolvedAgentProfile.InputSchema, encodedInput); err != nil {
			return Result{}, fmt.Errorf("validate AgentProfile input: %w", err)
		}
	}

	var toolCalls atomic.Int64
	var modelIteration atomic.Int64
	var toolBudgetExceeded atomic.Bool
	var startedToolCalls sync.Map
	state := newExecutionState()
	tools, err := r.agentTools(ctx, request, &toolCalls, &modelIteration, loopCancel, &toolBudgetExceeded, &startedToolCalls, state)
	if err != nil {
		return Result{}, err
	}
	toolErrorCallback := func(toolCtx agent.Context, failedTool tool.Tool, args map[string]any, toolErr error) (map[string]any, error) {
		// functiontool validates arguments before entering our handler. Such a
		// failure must still be visible in the same tool lifecycle as gateway
		// failures, while calls that reached the handler are already covered by
		// PolicyGateway and are marked in startedToolCalls.
		callID := ""
		if toolCtx != nil {
			callID = strings.TrimSpace(toolCtx.FunctionCallID())
		}
		handled := false
		if callID != "" {
			_, handled = startedToolCalls.LoadOrStore(callID, struct{}{})
		}
		name := ""
		if failedTool != nil {
			name = strings.TrimSpace(failedTool.Name())
		}
		binding := resolvedToolBinding(request, name)
		definition := binding.definition
		if definition.Name == "" {
			definition.Name = name
		}
		count := int64(toolCalls.Load())
		if !handled {
			count = toolCalls.Add(1)
		}
		call := ToolCall{
			ExecutionID: request.ExecutionID, Sequence: int(count), ActorID: request.ActorID,
			ScopeID: request.ScopeID, ResourceID: definition.ResourceID, Name: definition.Name,
			ModelToolName:      name,
			ProviderResourceID: request.AIProviderResourceID, ModelName: request.ModelName,
			Arguments: args, Iteration: int(modelIteration.Load()), CallID: callID,
		}
		message := publicError(toolErr)
		if message == "" {
			message = "工具参数校验失败"
		}
		code := classifyToolResponseError(message)
		if !handled && count > int64(request.Budget.MaxToolCalls) {
			message = "tool call budget exceeded"
			code = "tool_budget"
			toolBudgetExceeded.Store(true)
			if loopCancel != nil {
				loopCancel()
			}
		}
		completedAt := time.Now().UTC()
		if !handled && request.ToolGateway != nil {
			request.ToolGateway.recordToolCall(ToolCallRecord{
				ExecutionID: request.ExecutionID, Sequence: call.Sequence,
				ProviderResourceID: call.ProviderResourceID, ModelName: call.ModelName,
				ResourceID: call.ResourceID, ToolName: call.Name, Arguments: call.Arguments,
				Status: StatusFailed, ErrorCode: code, Error: message,
				StartedAt: completedAt, CompletedAt: completedAt,
			})
		}
		if !handled && request.EventSink != nil {
			_ = request.EventSink(Event{ExecutionID: request.ExecutionID, Type: "tool.requested", Status: StatusRunning, Payload: toolEventPayload(call, call.Name, map[string]any{"arguments": redactValue(args)})})
			_ = request.EventSink(Event{ExecutionID: request.ExecutionID, Type: "tool.failed", Status: StatusFailed, Payload: toolEventPayload(call, call.Name, map[string]any{
				"arguments": redactValue(args), "error": message, "error_code": code,
				"output": map[string]any{"error": message}, "duration_ms": int64(0),
			})})
		}
		// Return the same bounded, explicitly untrusted observation to ADK. This
		// becomes the FunctionResponse that the next model turn consumes, so a
		// validation/unknown-tool error can drive a correction instead of being
		// silently discarded.
		return map[string]any{
			"output": map[string]any{"error": message, "error_code": code},
			"error":  message, "error_code": code,
			"untrusted": true, "aiengine_observation": true,
		}, nil
	}
	beforeModel := func(_ agent.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
		if req == nil {
			return nil, nil
		}
		trimContext(req, modelResult.ContextWindowTokens)
		if req.Config == nil {
			req.Config = &genai.GenerateContentConfig{}
		}
		// Model configuration is authoritative for generation parameters. Keep
		// zero as a valid configured temperature (deterministic generation), so
		// do not use a non-zero check here.
		temperature := float32(modelResult.Temperature)
		req.Config.Temperature = &temperature
		stateText := state.prompt()
		if req.Config.SystemInstruction == nil {
			req.Config.SystemInstruction = genai.NewContentFromText(stateText, genai.RoleModel)
		} else {
			parts := make([]*genai.Part, 0, len(req.Config.SystemInstruction.Parts)+1)
			for _, part := range req.Config.SystemInstruction.Parts {
				if part != nil && !strings.HasPrefix(part.Text, "Execution state (machine-maintained") {
					parts = append(parts, part)
				}
			}
			parts = append(parts, &genai.Part{Text: stateText})
			req.Config.SystemInstruction.Parts = parts
		}
		maxOut := modelResult.MaxOutputTokens
		if requestedMaxOutputTokens > 0 {
			maxOut = requestedMaxOutputTokens
		}
		// The provider/model limit is authoritative. A request-level budget may
		// further reduce it, but can never increase it.
		if modelResult.MaxOutputTokens > 0 && (maxOut <= 0 || modelResult.MaxOutputTokens < maxOut) {
			maxOut = modelResult.MaxOutputTokens
		}
		if maxOut <= 0 {
			maxOut = 128000
		}
		if modelResult.ContextWindowTokens > 0 {
			remaining := modelResult.ContextWindowTokens - estimateRequestTokens(req) - 256
			if remaining < maxOut {
				maxOut = remaining
			}
		}
		if maxOut < 1 {
			maxOut = 1
		}
		req.Config.MaxOutputTokens = int32(maxOut)
		return nil, nil
	}
	agentRoot, err := llmagent.New(llmagent.Config{
		Name: "ai_engine_agent", Model: modelResult.Client, Instruction: instruction,
		Tools: tools, OnToolErrorCallbacks: []llmagent.OnToolErrorCallback{toolErrorCallback},
		BeforeModelCallbacks:     []llmagent.BeforeModelCallback{beforeModel},
		DisallowTransferToParent: true, DisallowTransferToPeers: true,
	})
	if err != nil {
		return Result{}, fmt.Errorf("create AIEngine agent: %w", err)
	}
	adkRunner, err := runner.NewInMemory(r.AppName, agentRoot)
	if err != nil {
		return Result{}, fmt.Errorf("create AIEngine runner: %w", err)
	}
	var output strings.Builder
	var promptTokens, completionTokens, totalTokens int64
	var iterations int
	modelTurnOpen := false
	var turnUsage *genai.GenerateContentResponseUsageMetadata
	turnUsageFinal := false
	var turnText strings.Builder
	var turnPartialText strings.Builder
	// Keep the exact visible source sent in assistant.delta events. This is
	// also the canonical source for the final result when streaming is active.
	var turnStreamText strings.Builder
	turnHasFunctionCall := false
	var turnToolNames []string
	startModelTurn := func() bool {
		iterations++
		turnText.Reset()
		turnPartialText.Reset()
		turnStreamText.Reset()
		turnHasFunctionCall = false
		turnToolNames = turnToolNames[:0]
		turnUsage = nil
		turnUsageFinal = false
		modelIteration.Store(int64(iterations))
		modelTurnOpen = true
		if iterations > request.Budget.MaxIterations {
			if sink != nil {
				_ = sink(Event{ExecutionID: request.ExecutionID, Type: "phase.changed", Status: StatusFailed, Payload: map[string]any{"phase": "budget_exceeded", "detail": "模型回合预算已用尽", "iteration": iterations, "reason": "iteration_budget"}})
			}
			return false
		}
		if sink != nil {
			_ = sink(Event{ExecutionID: request.ExecutionID, Type: "model.started", Status: StatusRunning, Payload: map[string]any{"iteration": iterations, "detail": "正在分析当前目标并评估下一步"}})
		}
		return true
	}
	commitTurnUsage := func() {
		if turnUsage != nil {
			promptTokens += int64(turnUsage.PromptTokenCount)
			completionTokens += int64(turnUsage.CandidatesTokenCount)
			totalTokens += int64(turnUsage.TotalTokenCount)
		}
		turnUsage = nil
		turnUsageFinal = false
	}
	// Emit the first model-turn boundary before entering ADK. This makes model
	// queue/network latency visible to streaming clients; subsequent turns are
	// opened when ADK yields the next model response.
	if !startModelTurn() {
		return Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "iteration_budget", ErrorMessage: "iteration budget exceeded", ToolCallCount: int(toolCalls.Load())}, errors.New("iteration budget exceeded")
	}
	for event, eventErr := range adkRunner.Run(ctx, request.ActorID, request.ExecutionID, genai.NewContentFromText(prompt, genai.RoleUser), agent.RunConfig{StreamingMode: streamingMode(request.Stream)}) {
		if eventErr != nil {
			if sinkErr := getSinkErr(); sinkErr != nil {
				return Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "event_sink", ErrorMessage: publicError(sinkErr), ToolCallCount: int(toolCalls.Load())}, sinkErr
			}
			if toolBudgetExceeded.Load() {
				return Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "tool_budget", ErrorMessage: "tool call budget exceeded", ToolCallCount: int(toolCalls.Load())}, errors.New("tool call budget exceeded")
			}
			return failedResult(request.ExecutionID, toolCalls.Load(), ctx, eventErr)
		}
		if event == nil {
			continue
		}
		if event.Content != nil && event.Content.Role == genai.RoleModel && !modelTurnOpen {
			if !startModelTurn() {
				return Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "iteration_budget", ErrorMessage: "iteration budget exceeded", ToolCallCount: int(toolCalls.Load())}, errors.New("iteration budget exceeded")
			}
		}
		// Streaming providers may attach the same (or cumulative) usage metadata
		// to several chunks. Keep the largest partial sample and replace it with
		// the final aggregate, then commit exactly once when the turn closes.
		if event.UsageMetadata != nil {
			if !event.Partial {
				turnUsage = event.UsageMetadata
				turnUsageFinal = true
			} else if !turnUsageFinal && (turnUsage == nil || event.UsageMetadata.TotalTokenCount >= turnUsage.TotalTokenCount) {
				turnUsage = event.UsageMetadata
			}
		}
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part == nil {
					continue
				}
				if part.FunctionCall != nil {
					turnHasFunctionCall = true
					if name := strings.TrimSpace(part.FunctionCall.Name); name != "" && !slices.Contains(turnToolNames, name) {
						turnToolNames = append(turnToolNames, name)
					}
				}
				// Thought parts are model-private reasoning. Do not expose or
				// persist them; only ordinary response text can be streamed.
				if !part.Thought && !event.Partial && part.Text != "" {
					turnText.WriteString(part.Text)
				}
				if event.Partial && !part.Thought && part.Text != "" {
					turnPartialText.WriteString(part.Text)
					if sink != nil {
						if text := safeStreamingDelta(part.Text, 0); text != "" {
							turnStreamText.WriteString(text)
							_ = sink(Event{ExecutionID: request.ExecutionID, Type: "assistant.delta", Status: StatusRunning, Payload: map[string]any{"text": text, "iteration": iterations}})
						}
					} else {
						turnStreamText.WriteString(part.Text)
					}
				}
			}
			if event.Content.Role == genai.RoleUser {
				// ADK handles unknown function names outside our registered
				// function tools. Surface its structured error response as an
				// Observation so the next model turn can correct the plan. A
				// response event may contain several independent function
				// responses; collect the whole batch and emit one model.resumed
				// boundary after all lifecycle events have been observed.
				observations := make([]map[string]any, 0)
				for _, part := range event.Content.Parts {
					if part == nil || part.FunctionResponse == nil {
						continue
					}
					functionResponse := part.FunctionResponse
					response := part.FunctionResponse.Response
					observations = append(observations, functionResponseObservation(request, functionResponse))

					// Errors returned by ADK itself (for example an unknown
					// function name) do not pass through our functiontool
					// callback. Reconstruct their lifecycle here. Ordinary
					// gateway/functiontool responses carry the marker below and
					// have already emitted their requested/failed events.
					if response == nil {
						continue
					}
					if handled, _ := response["aiengine_observation"].(bool); handled {
						continue
					}
					if functionResponse.ID != "" {
						if _, handled := startedToolCalls.Load(functionResponse.ID); handled {
							continue
						}
					}
					if _, failed := response["error"]; !failed {
						continue
					}
					modelToolName := strings.TrimSpace(functionResponse.Name)
					binding := resolvedToolBinding(request, modelToolName)
					toolName := modelToolName
					resourceID := ""
					if binding.definition.Name != "" {
						toolName = binding.definition.Name
						resourceID = binding.definition.ResourceID
					}
					call := ToolCall{
						ExecutionID:        request.ExecutionID,
						Sequence:           int(toolCalls.Add(1)),
						Iteration:          int(modelIteration.Load()),
						CallID:             functionResponse.ID,
						ActorID:            request.ActorID,
						ScopeID:            request.ScopeID,
						ResourceID:         resourceID,
						Name:               toolName,
						ModelToolName:      modelToolName,
						ProviderResourceID: request.AIProviderResourceID,
						ModelName:          request.ModelName,
					}
					message := publicError(fmt.Errorf("%v", response["error"]))
					if message == "" {
						message = "工具调用失败"
					}
					code := classifyToolResponseError(message)
					if int64(call.Sequence) > int64(request.Budget.MaxToolCalls) {
						message = "tool call budget exceeded"
						code = "tool_budget"
						toolBudgetExceeded.Store(true)
						loopCancel()
					}
					completedAt := time.Now().UTC()
					if request.ToolGateway != nil {
						request.ToolGateway.recordToolCall(ToolCallRecord{
							ExecutionID: call.ExecutionID, Sequence: call.Sequence,
							ProviderResourceID: call.ProviderResourceID, ModelName: call.ModelName,
							ResourceID: call.ResourceID, ToolName: call.Name, Arguments: call.Arguments,
							Status: StatusFailed, ErrorCode: code, Error: message,
							StartedAt: completedAt, CompletedAt: completedAt,
						})
					}
					if sink != nil {
						_ = sink(Event{ExecutionID: request.ExecutionID, Type: "tool.requested", Status: StatusRunning, Payload: toolEventPayload(call, call.Name, map[string]any{"arguments": redactValue(call.Arguments)})})
						_ = sink(Event{ExecutionID: request.ExecutionID, Type: "tool.failed", Status: StatusFailed, Payload: toolEventPayload(call, call.Name, map[string]any{
							"error": message, "error_code": code,
							"output": summarizeObservation(response), "duration_ms": int64(0),
						})})
					}
				}
				if len(observations) > 0 && sink != nil {
					emitModelResumed(sink, request.ExecutionID, iterations, observations)
				}
			}
			if !event.Partial {
				if turnHasFunctionCall {
					decisionText := strings.TrimSpace(turnText.String())
					if decisionText == "" {
						decisionText = strings.TrimSpace(turnPartialText.String())
					}
					decisionText = safeProgressText(decisionText, 2000)
					if decisionText == "" {
						if len(turnToolNames) > 0 {
							decisionText = "已确认需要调用 " + strings.Join(turnToolNames, "、") + "，准备获取环境信息"
						} else {
							decisionText = "已完成当前分析，准备调用受控工具"
						}
					}
					if sink != nil {
						_ = sink(Event{ExecutionID: request.ExecutionID, Type: "assistant.progress", Status: StatusRunning, Payload: map[string]any{"text": decisionText, "iteration": iterations, "kind": "tool_decision", "tool_count": len(turnToolNames), "parallel": len(turnToolNames) > 1}})
					}
				} else {
					// In SSE mode the partial chunks are the exact bytes already
					// delivered to clients. ADK also exposes a non-partial aggregate
					// for the same turn; using that aggregate as the durable answer
					// can produce a different Markdown source (most visibly around
					// line breaks). Keep one canonical source across live and history.
					answerText := turnPartialText.String()
					if sink != nil && turnStreamText.Len() > 0 {
						answerText = turnStreamText.String()
					}
					if answerText == "" {
						answerText = turnText.String()
					}
					if answerText != "" {
						output.WriteString(answerText)
					} else if sink != nil {
						_ = sink(Event{ExecutionID: request.ExecutionID, Type: "assistant.progress", Status: StatusRunning, Payload: map[string]any{"text": "正在分析当前目标并评估下一步", "iteration": iterations, "kind": "analysis"}})
					}
				}
				commitTurnUsage()
				modelTurnOpen = false
			}
		} else if modelTurnOpen && !event.Partial {
			// A provider may send a usage-only/turn-complete response after the
			// streamed text. Close the turn so the next model request receives a
			// fresh iteration and flush any text accumulated in partial chunks.
			if !turnHasFunctionCall && turnPartialText.Len() > 0 {
				output.WriteString(turnPartialText.String())
			}
			commitTurnUsage()
			modelTurnOpen = false
		}
		if totalTokens > request.Budget.MaxTokens {
			budgetDetail := fmt.Sprintf("模型累计 Token 预算已用尽（已使用 %d / %d；输入上下文 %d，模型输出 %d）", totalTokens, request.Budget.MaxTokens, promptTokens, completionTokens)
			if sink != nil {
				_ = sink(Event{ExecutionID: request.ExecutionID, Type: "phase.changed", Status: StatusFailed, Payload: map[string]any{"phase": "budget_exceeded", "detail": budgetDetail, "reason": "token_budget", "iteration": iterations, "used_tokens": totalTokens, "max_tokens": request.Budget.MaxTokens, "prompt_tokens": promptTokens, "completion_tokens": completionTokens}})
			}
			return Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "token_budget", ErrorMessage: budgetDetail, ToolCallCount: int(toolCalls.Load())}, errors.New(budgetDetail)
		}
	}
	if sinkErr := getSinkErr(); sinkErr != nil {
		return Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "event_sink", ErrorMessage: publicError(sinkErr), ToolCallCount: int(toolCalls.Load())}, sinkErr
	}
	if modelTurnOpen {
		if !turnHasFunctionCall {
			answerText := turnPartialText.String()
			if sink != nil && turnStreamText.Len() > 0 {
				answerText = turnStreamText.String()
			}
			if answerText == "" {
				answerText = turnText.String()
			}
			if answerText != "" {
				output.WriteString(answerText)
			}
		}
		commitTurnUsage()
	}
	if toolBudgetExceeded.Load() {
		return Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "tool_budget", ErrorMessage: "tool call budget exceeded", ToolCallCount: int(toolCalls.Load())}, errors.New("tool call budget exceeded")
	}
	if err := ctx.Err(); err != nil {
		code := "cancelled"
		if errors.Is(err, context.DeadlineExceeded) {
			code = "timeout"
		}
		return Result{ExecutionID: request.ExecutionID, Status: StatusCancelled, ErrorCode: code, ErrorMessage: publicError(err), ToolCallCount: int(toolCalls.Load())}, err
	}
	// The model's final text is returned to API callers and may be persisted by
	// a business adapter. Apply the same credential redaction used for streamed
	// and persisted assistant events before exposing it through Result.Output.
	text := scrubVisibleText(strings.TrimSpace(output.String()))
	if text == "" {
		if sink != nil {
			_ = sink(Event{ExecutionID: request.ExecutionID, Type: "phase.changed", Status: StatusFailed, Payload: map[string]any{"phase": "failed", "detail": "模型未返回可展示的最终回答", "reason": "empty_output", "iteration": iterations}})
		}
		return Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "empty_output", ErrorMessage: "model returned an empty final response", ToolCallCount: int(toolCalls.Load())}, errors.New("model returned an empty final response")
	}
	if len([]byte(text)) > request.Budget.MaxOutputBytes {
		if sink != nil {
			_ = sink(Event{ExecutionID: request.ExecutionID, Type: "phase.changed", Status: StatusFailed, Payload: map[string]any{"phase": "budget_exceeded", "detail": "最终输出大小超过预算", "reason": "output_budget", "iteration": iterations}})
		}
		return Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "output_budget", ErrorMessage: "output budget exceeded", ToolCallCount: int(toolCalls.Load())}, errors.New("output budget exceeded")
	}
	outputSchema := request.OutputSchema
	if len(outputSchema) == 0 && request.ResolvedAgentProfile != nil {
		outputSchema = request.ResolvedAgentProfile.OutputSchema
	}
	if len(outputSchema) > 0 {
		if err := validateJSON(outputSchema, []byte(text)); err != nil {
			return Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "agent_output_schema", ErrorMessage: publicError(err), ToolCallCount: int(toolCalls.Load())}, err
		}
	}
	if sink != nil {
		_ = sink(Event{ExecutionID: request.ExecutionID, Type: "assistant.completed", Status: StatusRunning, Payload: map[string]any{"text": scrubVisibleText(text), "iteration": iterations}})
		if sinkErr := getSinkErr(); sinkErr != nil {
			return Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "event_sink", ErrorMessage: publicError(sinkErr), ToolCallCount: int(toolCalls.Load())}, sinkErr
		}
	}
	return Result{ExecutionID: request.ExecutionID, Status: StatusSucceeded, Output: text, PromptTokens: promptTokens, CompletionTokens: completionTokens, TotalTokens: totalTokens, ToolCallCount: int(toolCalls.Load())}, nil
}

func (r *AgentRunner) agentTools(ctx context.Context, request Request, calls, iteration *atomic.Int64, stop context.CancelFunc, budgetExceeded *atomic.Bool, startedToolCalls *sync.Map, state *executionState) ([]tool.Tool, error) {
	result := make([]tool.Tool, 0)
	if request.ResolvedContext == nil || request.ToolGateway == nil {
		return result, nil
	}
	// A controlled paging tool lets the model retrieve large observations only
	// when needed, keeping the normal context compact.
	readTool, _ := functiontool.New(functiontool.Config{Name: "read_observation", Description: "按 observation_id、offset、limit 分页读取之前工具结果"}, func(toolCtx agent.Context, args map[string]any) (map[string]any, error) {
		count := calls.Add(1)
		if count > int64(request.Budget.MaxToolCalls) {
			if budgetExceeded != nil {
				budgetExceeded.Store(true)
			}
			if stop != nil {
				stop()
			}
			return map[string]any{"error": "tool call budget exceeded", "error_code": "tool_budget"}, nil
		}
		if request.EventSink != nil {
			_ = request.EventSink(Event{ExecutionID: request.ExecutionID, Type: "tool.requested", Status: StatusRunning, Payload: map[string]any{"tool": "read_observation", "call_id": toolCtx.FunctionCallID(), "iteration": iteration.Load()}})
		}
		id, _ := args["observation_id"].(string)
		number := func(key string) int {
			switch value := args[key].(type) {
			case int:
				return value
			case int64:
				return int(value)
			case float64:
				return int(value)
			case json.Number:
				n, _ := value.Int64()
				return int(n)
			}
			return 0
		}
		offset, limit := number("offset"), number("limit")
		value, ok := state.read(id, offset, limit)
		if !ok {
			if request.EventSink != nil {
				_ = request.EventSink(Event{ExecutionID: request.ExecutionID, Type: "tool.failed", Status: StatusFailed, Payload: map[string]any{"tool": "read_observation", "error": "observation not found", "error_code": "observation_not_found", "iteration": iteration.Load()}})
			}
			return map[string]any{"error": "observation not found"}, nil
		}
		if request.EventSink != nil {
			_ = request.EventSink(Event{ExecutionID: request.ExecutionID, Type: "tool.completed", Status: StatusSucceeded, Payload: map[string]any{"tool": "read_observation", "call_id": toolCtx.FunctionCallID(), "iteration": iteration.Load(), "output": value}})
		}
		return map[string]any{"output": value, "aiengine_observation": true}, nil
	})
	if readTool != nil && state != nil {
		result = append(result, readTool)
	}
	for _, binding := range buildToolBindings(request.ResolvedContext.Tools) {
		definition := binding.definition
		modelToolName := binding.modelName
		if definition.Name == "" || definition.ResourceID == "" {
			continue
		}
		if !agentToolAllowed(request.ResolvedAgentProfile, request.AllowedTools, request.RestrictTools, definition.Name) {
			continue
		}
		var inputSchema *adkjsonschema.Schema
		if len(definition.InputSchema) > 0 {
			var parsed adkjsonschema.Schema
			if err := json.Unmarshal(definition.InputSchema, &parsed); err != nil {
				return nil, fmt.Errorf("tool %s has invalid input schema: %w", definition.Name, err)
			}
			inputSchema = &parsed
		}
		item, err := functiontool.New(functiontool.Config{Name: modelToolName, Description: definition.Description, InputSchema: inputSchema}, func(toolCtx agent.Context, args map[string]any) (map[string]any, error) {
			count := calls.Add(1)
			call := ToolCall{ExecutionID: request.ExecutionID, Sequence: int(count), ActorID: request.ActorID, ScopeID: request.ScopeID, ResourceID: definition.ResourceID, Name: definition.Name, ModelToolName: modelToolName, ProviderResourceID: request.AIProviderResourceID, ModelName: request.ModelName, Arguments: args, Iteration: int(iteration.Load()), CallID: toolCtx.FunctionCallID(), EventSink: request.EventSink}
			if startedToolCalls != nil && call.CallID != "" {
				startedToolCalls.Store(call.CallID, struct{}{})
			}
			if count > int64(request.Budget.MaxToolCalls) {
				message := "tool call budget exceeded"
				completedAt := time.Now().UTC()
				request.ToolGateway.recordToolCall(ToolCallRecord{
					ExecutionID: call.ExecutionID, Sequence: call.Sequence,
					ProviderResourceID: call.ProviderResourceID, ModelName: call.ModelName,
					ResourceID: call.ResourceID, ToolName: call.Name, Arguments: call.Arguments,
					Status: StatusFailed, ErrorCode: "tool_budget", Error: message,
					StartedAt: completedAt, CompletedAt: completedAt,
				})
				if request.EventSink != nil {
					_ = request.EventSink(Event{ExecutionID: request.ExecutionID, Type: "tool.requested", Status: StatusRunning, Payload: toolEventPayload(call, definition.Name, map[string]any{"arguments": redactValue(args)})})
					_ = request.EventSink(Event{ExecutionID: request.ExecutionID, Type: "tool.failed", Status: StatusFailed, Payload: toolEventPayload(call, definition.Name, map[string]any{
						"arguments":   redactValue(args),
						"error":       message,
						"error_code":  "tool_budget",
						"output":      map[string]any{"error": message},
						"duration_ms": int64(0),
					})})
				}
				if budgetExceeded != nil && budgetExceeded.CompareAndSwap(false, true) {
					if request.EventSink != nil {
						_ = request.EventSink(Event{ExecutionID: request.ExecutionID, Type: "phase.changed", Status: StatusFailed, Payload: map[string]any{"phase": "budget_exceeded", "detail": "工具调用预算已用尽", "reason": "tool_budget", "iteration": iteration.Load(), "tool_call_count": count}})
					}
				}
				if stop != nil {
					stop()
				}
				return nil, ErrToolLimited
			}
			// Preserve ADK's per-call cancellation/deadline and invocation values
			// when entering the gateway. The outer loop context remains the hard
			// execution budget, while toolCtx carries any tool-specific cancellation.
			toolResult, invokeErr := request.ToolGateway.Invoke(toolCtx, call)
			if invokeErr != nil {
				// Return a structured, sanitized observation instead of exposing the
				// raw provider error through ADK's function-response path. ADK will
				// still continue the loop and let the model choose a correction.
				observation := map[string]any{"error": publicError(invokeErr), "untrusted": true, "aiengine_observation": true}
				return observation, nil
			}
			if request.ObservationSink != nil {
				request.ObservationSink(ToolObservation{ToolName: definition.Name, ResourceID: definition.ResourceID, Iteration: call.Iteration, CallID: call.CallID, Result: toolResult})
			}
			observation := summarizeObservation(toolResult.Output)
			// The replay store is still bounded by execution lifetime, but stores
			// the complete recursively redacted value so paging never reintroduces
			// credentials that were removed from the model-facing summary.
			observationID := state.save(scrubObservationText(redactValue(toolResult.Output)))
			state.observe(definition.Name, toolResult.Output)
			// Preserve the original result for Evidence/audit sinks, but pass only
			// the bounded, recursively redacted observation to the model.
			return map[string]any{"output": map[string]any{"observation_id": observationID, "summary": observation, "read_tool": "read_observation"}, "untrusted": toolResult.Untrusted, "truncated": toolResult.Truncated, "aiengine_observation": true}, nil
		})
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

// functionResponseObservation converts one ADK FunctionResponse into the
// bounded observation shape used by the execution timeline. ADK can merge
// several independent responses into one user event, so this helper keeps the
// canonical tool/resource identity alongside the model-facing alias.
func functionResponseObservation(request Request, response *genai.FunctionResponse) map[string]any {
	item := map[string]any{"outcome": "success"}
	if response == nil {
		item["observation"] = summarizeObservation(nil)
		return item
	}
	modelToolName := strings.TrimSpace(response.Name)
	binding := resolvedToolBinding(request, modelToolName)
	toolName := modelToolName
	resourceID := ""
	if binding.definition.Name != "" {
		toolName = binding.definition.Name
		resourceID = binding.definition.ResourceID
	}
	if toolName != "" {
		item["tool"] = toolName
	}
	if modelToolName != "" && modelToolName != toolName {
		item["model_tool"] = modelToolName
	}
	if resourceID != "" {
		item["resource_id"] = resourceID
	}
	if response.ID != "" {
		item["call_id"] = response.ID
	}

	var observation any
	if response.Response != nil {
		observation = response.Response
		if output, ok := response.Response["output"]; ok {
			observation = output
		}
		if _, failed := response.Response["error"]; failed {
			item["outcome"] = "error"
		}
	}
	item["observation"] = summarizeObservation(observation)
	return item
}

// emitModelResumed marks the barrier between a completed tool batch and the
// next model turn. It intentionally emits one event for the whole batch: ADK
// executes same-turn independent calls concurrently, and the model should
// reason over all of their observations together. A single-call payload keeps
// the historical fields for clients that only render one tool; multi-call
// payloads additionally expose the ordered observation list and parallel flag.
func emitModelResumed(sink EventSink, executionID string, iteration int, observations []map[string]any) {
	if sink == nil || len(observations) == 0 {
		return
	}
	anyError := false
	allError := true
	for _, observation := range observations {
		if strings.EqualFold(fmt.Sprint(observation["outcome"]), "error") {
			anyError = true
		} else {
			allError = false
		}
	}
	outcome := "success"
	if allError {
		outcome = "error"
	} else if anyError {
		outcome = "partial"
	}
	payload := map[string]any{
		"iteration":    iteration,
		"tool_count":   len(observations),
		"parallel":     len(observations) > 1,
		"observations": observations,
		"outcome":      outcome,
	}
	if len(observations) == 1 {
		for _, key := range []string{"tool", "model_tool", "resource_id", "call_id", "observation"} {
			if value, ok := observations[0][key]; ok {
				payload[key] = value
			}
		}
	} else {
		payload["observation"] = map[string]any{"tools": observations, "count": len(observations)}
	}
	_ = sink(Event{ExecutionID: executionID, Type: "model.resumed", Status: StatusRunning, Payload: payload})
}

func agentToolAllowed(profile *AgentProfile, declared []string, restricted bool, name string) bool {
	if len(declared) > 0 {
		allowed := false
		for _, item := range declared {
			if strings.TrimSpace(item) == name {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	} else if restricted {
		return false
	}
	if profile == nil || len(profile.AllowedTools) == 0 {
		return true
	}
	for _, allowed := range profile.AllowedTools {
		if strings.TrimSpace(allowed) == name {
			return true
		}
	}
	return false
}

func resolvedToolBinding(request Request, name string) toolBinding {
	for _, binding := range buildToolBindings(resolvedToolDefinitions(request)) {
		if binding.modelName == name {
			return binding
		}
	}
	// A raw name remains addressable for backwards-compatible providers and
	// unknown-tool diagnostics. Only choose it when it is not ambiguous.
	var match toolBinding
	for _, binding := range buildToolBindings(resolvedToolDefinitions(request)) {
		if binding.definition.Name != name {
			continue
		}
		if match.definition.Name != "" {
			return toolBinding{}
		}
		match = binding
	}
	return match
}

func resolvedToolDefinitions(request Request) []ToolDefinition {
	if request.ResolvedContext == nil {
		return nil
	}
	return request.ResolvedContext.Tools
}

func resolvedToolDefinition(request Request, name string) ToolDefinition {
	return resolvedToolBinding(request, name).definition
}

func executionPrompt(request Request) (string, error) {
	payload := make(map[string]any, len(request.Input)+3)
	for key, value := range request.Input {
		payload[key] = value
	}
	if request.Task != "" {
		payload["task"] = request.Task
	}
	if len(request.Messages) > 0 {
		payload["conversation"] = request.Messages
	}
	// Context providers may collect a small, read-only snapshot while resolving
	// the selected resources (for example a PostgreSQL health check or MCP
	// server metadata). Make that environment feedback available on the first
	// model turn as well as subsequent tool observations. It is normalized,
	// redacted and bounded by summarizeObservation because connector/MCP data
	// is untrusted and may contain credentials or unexpectedly large payloads.
	if request.ResolvedContext != nil && len(request.ResolvedContext.Facts) > 0 {
		payload["environment_facts"] = summarizeObservation(request.ResolvedContext.Facts)
	}
	if len(payload) == 0 {
		return "请完成当前任务。", nil
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("AIEngine input must be JSON serializable: %w", err)
	}
	return string(encoded), nil
}

func failedResult(executionID string, calls int64, ctx context.Context, err error) (Result, error) {
	code := "agent"
	status := StatusFailed
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		code, status = "timeout", StatusCancelled
	} else if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		code, status = "cancelled", StatusCancelled
	}
	return Result{ExecutionID: executionID, Status: status, ErrorCode: code, ErrorMessage: publicError(err), ToolCallCount: int(calls)}, err
}

func missingCapabilities(actual, required []string) []string {
	missing := make([]string, 0)
	for _, value := range required {
		value = strings.TrimSpace(value)
		if value == "" || slices.Contains(actual, value) || slices.Contains(missing, value) {
			continue
		}
		missing = append(missing, value)
	}
	return missing
}

func validateJSON(schemaRaw json.RawMessage, value []byte) error {
	var schemaValue any
	if err := json.Unmarshal(schemaRaw, &schemaValue); err != nil {
		return err
	}
	var data any
	if err := json.Unmarshal(value, &data); err != nil {
		return errors.New("value must be valid JSON")
	}
	compiler := tekurischema.NewCompiler()
	if err := compiler.AddResource("memory://aiengine-schema.json", schemaValue); err != nil {
		return err
	}
	schema, err := compiler.Compile("memory://aiengine-schema.json")
	if err != nil {
		return err
	}
	return schema.Validate(data)
}

func publicError(err error) string {
	if err == nil {
		return ""
	}
	message := scrubVisibleText(strings.TrimSpace(err.Error()))
	return truncateUTF8(message, 1000)
}

// SafeErrorMessage returns the bounded, credential-redacted form suitable for
// API responses and user-visible execution events. Internal callers should
// still retain the original error for classification and logging.
func SafeErrorMessage(err error) string {
	return publicError(err)
}

func classifyToolResponseError(message string) string {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "not found"):
		return "tool_not_found"
	case strings.Contains(lower, "denied"), strings.Contains(lower, "forbidden"):
		return "tool_denied"
	case strings.Contains(lower, "invalid"), strings.Contains(lower, "argument"), strings.Contains(lower, "required"), strings.Contains(lower, "validating"):
		return "tool_validation"
	default:
		return "tool_execution"
	}
}

// summarizeObservation creates safe, bounded environment feedback for the UI
// and the next model turn. Tool output is untrusted and may contain secrets or
// very large logs, so redact recursively and cap the serialized representation.
func summarizeObservation(value any) any {
	// Normalize through JSON so typed maps/slices returned by an MCP adapter
	// receive the same recursive redaction as map[string]any values.
	normalized := value
	if encoded, err := json.Marshal(value); err == nil {
		var generic any
		if json.Unmarshal(encoded, &generic) == nil {
			normalized = generic
		}
	}
	redacted := scrubObservationText(redactValue(normalized))
	encoded, err := json.Marshal(redacted)
	if err != nil {
		return "工具返回了无法展示的结果"
	}
	if len(encoded) <= maxObservationBytes {
		return redacted
	}
	if object, ok := redacted.(map[string]any); ok {
		// Preserve diagnostic signal before verbose payloads such as logs or
		// query rows. The omitted fields remain available through observation
		// paging when the model needs exact details.
		priority := make(map[string]any)
		for key, item := range object {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "error") || strings.Contains(lower, "status") || strings.Contains(lower, "state") || strings.Contains(lower, "count") || strings.Contains(lower, "total") || strings.Contains(lower, "message") || strings.Contains(lower, "duration") {
				priority[key] = item
			}
		}
		if len(priority) > 0 {
			encodedPriority, _ := json.Marshal(priority)
			if len(encodedPriority) < maxObservationBytes {
				priority["details_truncated"] = true
				return priority
			}
		}
	}
	return map[string]any{"summary": truncateBytesUTF8(string(encoded), maxObservationBytes), "truncated": true}
}

// Tool observations are bounded independently from the provider output budget.
// The larger bound allows docker logs and similar diagnostics to retain a
// useful 200-line window while still preventing an unbounded tool response from
// consuming the model context. Exact values remain available through the
// read_observation paging tool.
const maxObservationBytes = 256 << 10
const maxObservationTextRunes = 64000

var sensitiveProgressPattern = regexp.MustCompile(`(?i)(\b(?:bearer|token|password|secret|credential|api[\s_-]*key|authorization|private[\s_-]*key|access[\s_-]*key|client[\s_-]*secret)\b\s*(?:[:=]\s*))(?:bearer\s+)?[^\s,;]+`)
var sensitiveJSONPattern = regexp.MustCompile(`(?i)(["']?(?:bearer|token|password|secret|credential|api[\s_-]*key|authorization|private[\s_-]*key|access[\s_-]*key|client[\s_-]*secret)["']?\s*:\s*)("(?:\\.|[^"\\])*"|[^,\s}\]]+)`)

func safeProgressText(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = sensitiveProgressPattern.ReplaceAllString(value, `$1[REDACTED]`)
	return truncateUTF8(value, maxRunes)
}

func safeStreamingDelta(value string, maxRunes int) string {
	// Whitespace is meaningful in Markdown (for example the space after `##`
	// and the blank line separating a heading from its paragraph). Only an
	// actually empty provider chunk may be discarded.
	if value == "" {
		return ""
	}
	value = sensitiveProgressPattern.ReplaceAllString(value, `$1[REDACTED]`)
	if maxRunes <= 0 {
		return value
	}
	return truncateUTF8(value, maxRunes)
}

func scrubVisibleText(value string) string {
	value = sensitiveProgressPattern.ReplaceAllString(value, `$1[REDACTED]`)
	value = sensitiveJSONPattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := sensitiveJSONPattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return `[REDACTED]`
		}
		replacement := `[REDACTED]`
		if strings.HasPrefix(strings.TrimSpace(parts[2]), `"`) {
			replacement = `"[REDACTED]"`
		}
		return parts[1] + replacement
	})
	return value
}

func truncateUTF8(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "…"
}

func truncateBytesUTF8(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len([]byte(value)) <= maxBytes {
		return value
	}
	ellipsis := []byte("…")
	if maxBytes < len(ellipsis) {
		bytes := []byte(value)
		limit := maxBytes
		for limit > 0 && !utf8.Valid(bytes[:limit]) {
			limit--
		}
		return string(bytes[:limit])
	}
	limit := maxBytes - len(ellipsis)
	bytes := []byte(value)
	for limit > 0 && !utf8.Valid(bytes[:limit]) {
		limit--
	}
	return string(bytes[:limit]) + string(ellipsis)
}

func scrubObservationText(value any) any {
	switch item := value.(type) {
	case string:
		return safeProgressText(item, maxObservationTextRunes)
	case map[string]any:
		result := make(map[string]any, len(item))
		for key, nested := range item {
			result[key] = scrubObservationText(nested)
		}
		return result
	case []any:
		result := make([]any, len(item))
		for index, nested := range item {
			result[index] = scrubObservationText(nested)
		}
		return result
	default:
		return value
	}
}

// estimateRequestTokens intentionally uses a conservative character heuristic
// because providers do not expose a tokenizer through the neutral ADK API.
func estimateRequestTokens(req *model.LLMRequest) int {
	if req == nil {
		return 0
	}
	b, _ := json.Marshal(req.Contents)
	if req.Config != nil {
		c, _ := json.Marshal(req.Config)
		b = append(b, c...)
	}
	return (len(b) + 3) / 4
}

// trimContext keeps the system prompt, the initial user task, and the latest
// complete turns. Function-call/response pairs are retained as atomic groups
// so providers never receive an invalid tool conversation.
func trimContext(req *model.LLMRequest, window int) {
	if req == nil || window <= 0 || len(req.Contents) < 4 {
		return
	}
	maxBytes := (window - 512) * 4
	if maxBytes <= 0 {
		return
	}
	contents := req.Contents
	keep := make([]*genai.Content, 0, len(contents))
	if len(contents) > 0 {
		keep = append(keep, contents[0])
	}
	start := len(contents) - 1
	used, _ := json.Marshal(keep)
	for start >= 1 {
		candidate := contents[start]
		groupStart := start
		// Keep function call and its following response together.
		if candidate != nil && candidate.Role == genai.RoleUser && start > 0 && contents[start-1] != nil && contents[start-1].Role == genai.RoleModel {
			groupStart = start - 1
		}
		group := contents[groupStart : start+1]
		encoded, _ := json.Marshal(group)
		if len(used)+len(encoded) > maxBytes {
			break
		}
		combined := make([]*genai.Content, 0, len(group)+len(keep))
		combined = append(combined, group...)
		combined = append(combined, keep...)
		keep = combined
		used = append(used, encoded...)
		start = groupStart - 1
	}
	if len(keep) < len(contents) {
		keep = append([]*genai.Content{genai.NewContentFromText("Earlier conversation and tool observations were compacted. Use read_observation with an observation_id when exact details are required.", genai.RoleUser)}, keep...)
	}
	req.Contents = keep
}

func streamingMode(stream bool) agent.StreamingMode {
	if stream {
		return agent.StreamingModeSSE
	}
	return agent.StreamingModeNone
}
