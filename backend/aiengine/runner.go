package aiengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	adkjsonschema "github.com/google/jsonschema-go/jsonschema"
	tekurischema "github.com/santhosh-tekuri/jsonschema/v6"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/genai"
)

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
	return r.execute(ctx, request, sink)
}

func (r *AgentRunner) execute(parent context.Context, request Request, sink EventSink) (Result, error) {
	if r == nil || r.BuildModel == nil {
		return Result{}, ErrRunnerUnavailable
	}
	if parent == nil {
		parent = context.Background()
	}
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
	if request.Budget.MaxOutputBytes <= 0 {
		request.Budget.MaxOutputBytes = DefaultBudget().MaxOutputBytes
	}
	ctx, cancel := context.WithTimeout(parent, request.Budget.Timeout)
	defer cancel()
	if sink != nil {
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
	tools, err := r.agentTools(ctx, request, &toolCalls)
	if err != nil {
		return Result{}, err
	}
	agentRoot, err := llmagent.New(llmagent.Config{
		Name: "ai_engine_agent", Model: modelResult.Client, Instruction: instruction,
		Tools: tools, DisallowTransferToParent: true, DisallowTransferToPeers: true,
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
	for event, eventErr := range adkRunner.Run(ctx, request.ActorID, request.ExecutionID, genai.NewContentFromText(prompt, genai.RoleUser), agent.RunConfig{StreamingMode: streamingMode(request.Stream)}) {
		if eventErr != nil {
			return failedResult(request.ExecutionID, toolCalls.Load(), ctx, eventErr)
		}
		if event == nil {
			continue
		}
		if event.Content != nil && !event.Partial && event.Content.Role == genai.RoleModel {
			iterations++
			if iterations > request.Budget.MaxIterations {
				return Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "iteration_budget", ErrorMessage: "iteration budget exceeded", ToolCallCount: int(toolCalls.Load())}, errors.New("iteration budget exceeded")
			}
		}
		if event.UsageMetadata != nil {
			promptTokens += int64(event.UsageMetadata.PromptTokenCount)
			completionTokens += int64(event.UsageMetadata.CandidatesTokenCount)
			totalTokens += int64(event.UsageMetadata.TotalTokenCount)
		}
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part == nil {
					continue
				}
				if event.Partial && part.Text != "" {
					output.WriteString(part.Text)
					if sink != nil {
						_ = sink(Event{ExecutionID: request.ExecutionID, Type: "assistant.delta", Status: StatusRunning, Payload: map[string]any{"text": part.Text}})
					}
				} else if !event.Partial && part.Text != "" {
					if output.Len() == 0 {
						output.WriteString(part.Text)
					}
				}
			}
		}
		if totalTokens > request.Budget.MaxTokens {
			return Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "token_budget", ErrorMessage: "token budget exceeded", ToolCallCount: int(toolCalls.Load())}, errors.New("token budget exceeded")
		}
	}
	if err := ctx.Err(); err != nil {
		code := "cancelled"
		if errors.Is(err, context.DeadlineExceeded) {
			code = "timeout"
		}
		return Result{ExecutionID: request.ExecutionID, Status: StatusCancelled, ErrorCode: code, ErrorMessage: err.Error(), ToolCallCount: int(toolCalls.Load())}, err
	}
	text := strings.TrimSpace(output.String())
	if len([]byte(text)) > request.Budget.MaxOutputBytes {
		return Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "output_budget", ErrorMessage: "output budget exceeded", ToolCallCount: int(toolCalls.Load())}, errors.New("output budget exceeded")
	}
	outputSchema := request.OutputSchema
	if len(outputSchema) == 0 && request.ResolvedAgentProfile != nil {
		outputSchema = request.ResolvedAgentProfile.OutputSchema
	}
	if len(outputSchema) > 0 {
		if err := validateJSON(outputSchema, []byte(text)); err != nil {
			return Result{ExecutionID: request.ExecutionID, Status: StatusFailed, ErrorCode: "agent_output_schema", ErrorMessage: err.Error(), ToolCallCount: int(toolCalls.Load())}, err
		}
	}
	if sink != nil {
		_ = sink(Event{ExecutionID: request.ExecutionID, Type: "assistant.completed", Status: StatusRunning, Payload: map[string]any{"text": text}})
	}
	return Result{ExecutionID: request.ExecutionID, Status: StatusSucceeded, Output: text, PromptTokens: promptTokens, CompletionTokens: completionTokens, TotalTokens: totalTokens, ToolCallCount: int(toolCalls.Load())}, nil
}

func (r *AgentRunner) agentTools(ctx context.Context, request Request, calls *atomic.Int64) ([]tool.Tool, error) {
	result := make([]tool.Tool, 0)
	if request.ResolvedContext == nil || request.ToolGateway == nil {
		return result, nil
	}
	for _, definition := range request.ResolvedContext.Tools {
		definition := definition
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
		item, err := functiontool.New(functiontool.Config{Name: definition.Name, Description: definition.Description, InputSchema: inputSchema}, func(_ agent.Context, args map[string]any) (map[string]any, error) {
			if calls.Add(1) > int64(request.Budget.MaxToolCalls) {
				return nil, ErrToolLimited
			}
			call := ToolCall{ExecutionID: request.ExecutionID, ActorID: request.ActorID, ScopeID: request.ScopeID, ResourceID: definition.ResourceID, Name: definition.Name, ProviderResourceID: request.AIProviderResourceID, ModelName: request.ModelName, Arguments: args, EventSink: request.EventSink}
			toolResult, invokeErr := request.ToolGateway.Invoke(ctx, call)
			if invokeErr != nil {
				return nil, invokeErr
			}
			if request.ObservationSink != nil {
				request.ObservationSink(ToolObservation{ToolName: definition.Name, ResourceID: definition.ResourceID, Result: toolResult})
			}
			return map[string]any{"output": toolResult.Output, "untrusted": toolResult.Untrusted}, nil
		})
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
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
	message := strings.TrimSpace(err.Error())
	if len(message) > 1000 {
		return message[:1000]
	}
	return message
}

func streamingMode(stream bool) agent.StreamingMode {
	if stream {
		return agent.StreamingModeSSE
	}
	return agent.StreamingModeNone
}
