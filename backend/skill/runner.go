package skill

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/genai"
	"opskeeper/backend/aiengine"

	"opskeeper/backend/authorization"
	"opskeeper/backend/connector"
	"opskeeper/backend/llm"
	"opskeeper/backend/observability"
)

type Connector interface {
	QueryMetrics(context.Context, string, connector.MetricsQuery) (connector.Evidence, error)
	QueryLogs(context.Context, string, connector.LogsQuery) (connector.Evidence, error)
	QueryTraces(context.Context, string, connector.TracesQuery) (connector.Evidence, error)
	GetAlerts(context.Context, string, connector.AlertsQuery) (connector.Evidence, error)
	ReadKubernetes(context.Context, string, connector.KubernetesQuery) (connector.Evidence, error)
	InspectPostgreSQL(context.Context, string) (connector.Evidence, error)
	InspectRedis(context.Context, string) (connector.Evidence, error)
	InspectKafka(context.Context, string) (connector.Evidence, error)
}

type RunInput struct {
	ExecutionID, ActorID, ScopeID, TargetResourceID string
	SkillResourceID, SkillVersionID                 string
	AIProviderResourceID, ModelName                 string
	Purpose                                         aiengine.Purpose
	AgentProfileID                                  string
	AgentProfile                                    *aiengine.AgentProfile
	RequiredCapabilities                            []string
	Input                                           map[string]any
	MaxIterations                                   int
	MaxToolCalls                                    int
	MaxTokens                                       int64
	MaxOutputBytes                                  int
	Timeout                                         time.Duration
	Stream                                          bool
	EvidenceObserver                                func(ObservedEvidence)
	EventSink                                       aiengine.EventSink
	ToolCallAudit                                   aiengine.ToolCallStore
	ResolvedContext                                 *aiengine.ResolvedContext
	ToolGateway                                     *aiengine.PolicyGateway
}

// ObservedEvidence is emitted only after a Connector has returned a typed,
// read-only result through the same policy-checked Tool invocation.
type ObservedEvidence struct {
	ToolName, TargetResourceID string
	Evidence                   connector.Evidence
}

type RunResult struct {
	Execution Execution `json:"execution"`
	Output    string    `json:"output"`
	Events    int       `json:"events"`
}

type Runner struct {
	Skills        *Service
	Models        *llm.Service
	Connector     Connector
	Executions    Store
	AgentProfiles aiengine.AgentProfileResolver
	AppName       string
}

func NewRunner(skills *Service, models *llm.Service, connectorService Connector, executions Store) *Runner {
	return &Runner{Skills: skills, Models: models, Connector: connectorService, Executions: executions, AppName: "opskeeper-skill"}
}

func (r *Runner) WithAgentProfileResolver(resolver aiengine.AgentProfileResolver) *Runner {
	r.AgentProfiles = resolver
	return r
}

func (r *Runner) Run(ctx context.Context, input RunInput) (RunResult, error) {
	if r.Skills == nil || r.Models == nil || (r.Executions == nil && strings.TrimSpace(input.SkillResourceID) != "") {
		return RunResult{}, errors.New("Skill Runner dependencies are unavailable")
	}
	if input.Timeout <= 0 {
		input.Timeout = 2 * time.Minute
	}
	if input.MaxToolCalls <= 0 {
		input.MaxToolCalls = 12
	}
	if input.MaxIterations <= 0 {
		input.MaxIterations = 12
	}
	if input.MaxOutputBytes <= 0 {
		input.MaxOutputBytes = 64 << 10
	}
	if input.MaxTokens <= 0 {
		input.MaxTokens = 20000
	}
	ctx, cancel := context.WithTimeout(ctx, input.Timeout)
	defer cancel()
	if input.AgentProfile == nil && strings.TrimSpace(input.AgentProfileID) != "" {
		if r.AgentProfiles == nil {
			return RunResult{}, invalid("agent profile resolver is unavailable")
		}
		profile, profileErr := r.AgentProfiles.Resolve(ctx, input.ScopeID, input.AgentProfileID)
		if profileErr != nil {
			return RunResult{}, profileErr
		}
		input.AgentProfile = &profile
	}
	if input.AgentProfile != nil {
		if profileErr := validateAgentProfile(*input.AgentProfile, input.ScopeID); profileErr != nil {
			return RunResult{}, profileErr
		}
	}
	version, err := r.resolveVersion(ctx, input)
	if err != nil {
		return RunResult{}, err
	}
	if input.TargetResourceID != "" && len(version.Manifest.TargetKinds) > 0 {
		if _, err := r.Skills.ValidateTarget(ctx, version, input.TargetResourceID); err != nil {
			return RunResult{}, err
		}
	}
	resolved, modelClient, err := r.Models.BuildModel(ctx, input.ScopeID, input.AIProviderResourceID, input.ModelName, purposeFromEngine(input.Purpose))
	if err != nil {
		return RunResult{}, err
	}
	requiredCapabilities := append([]string(nil), input.RequiredCapabilities...)
	if input.AgentProfile != nil {
		requiredCapabilities = append(requiredCapabilities, input.AgentProfile.Capabilities...)
	}
	if missing := missingCapabilities(resolved.Model.Capabilities, requiredCapabilities); len(missing) > 0 {
		return RunResult{}, invalid("AgentProfile requires model capabilities: " + strings.Join(missing, ", "))
	}
	encodedInput, err := json.Marshal(input.Input)
	if err != nil {
		return RunResult{}, invalid("Skill input must be JSON serializable")
	}
	if err := validateJSON(version.InputSchema, encodedInput); err != nil {
		return RunResult{}, fmt.Errorf("validate Skill input: %w", err)
	}
	if input.AgentProfile != nil && len(input.AgentProfile.InputSchema) > 0 {
		if err := validateJSON(input.AgentProfile.InputSchema, encodedInput); err != nil {
			return RunResult{}, fmt.Errorf("validate AgentProfile input: %w", err)
		}
	}
	digest := sha256.Sum256(encodedInput)
	persistExecution := version.SkillResourceID != "" && version.ID != ""
	var execution Execution
	if persistExecution {
		execution, err = r.Executions.StartExecution(ctx, StartExecutionInput{ScopeID: input.ScopeID, ActorUserID: input.ActorID, TargetResourceID: input.TargetResourceID, SkillResourceID: version.SkillResourceID, SkillVersionID: version.ID, ProviderResourceID: resolved.Provider.ResourceID, ModelName: resolved.Model.Name, InputDigest: hex.EncodeToString(digest[:])})
		if err != nil {
			return RunResult{}, err
		}
	} else {
		execution = Execution{ID: input.ExecutionID, ScopeID: input.ScopeID, ProviderResourceID: resolved.Provider.ResourceID, ModelName: resolved.Model.Name, Status: "running", InputDigest: hex.EncodeToString(digest[:])}
		if execution.ID == "" {
			execution.ID = fmt.Sprintf("exec-%d", time.Now().UnixNano())
		}
	}
	finish := func(result FinishExecutionInput) (RunResult, error) {
		observability.RecordLLM(context.Background(), result.Status, result.TotalTokens)
		if result.Status != "succeeded" {
			observability.RecordError(context.Background(), "llm", result.ErrorCode)
		}
		if !persistExecution {
			execution.Status, execution.OutputPreview = result.Status, result.OutputPreview
			execution.PromptTokens, execution.CompletionTokens, execution.TotalTokens = result.PromptTokens, result.CompletionTokens, result.TotalTokens
			execution.ToolCallCount, execution.ErrorCode, execution.ErrorMessage = result.ToolCallCount, result.ErrorCode, result.ErrorMessage
			now := time.Now().UTC()
			execution.CompletedAt = &now
			return RunResult{Execution: execution, Output: result.OutputPreview}, nil
		}
		completed, finishErr := r.Executions.FinishExecution(context.Background(), execution.ID, result)
		if finishErr != nil {
			return RunResult{}, finishErr
		}
		return RunResult{Execution: completed, Output: result.OutputPreview}, nil
	}
	if err := ctx.Err(); err != nil {
		return finish(FinishExecutionInput{Status: "cancelled", ErrorCode: "timeout", ErrorMessage: "execution timed out"})
	}

	resourceFilter, _ := authorization.ResourceFilterFromContext(ctx)
	policy := &policy{runner: r, runCtx: ctx, input: input, version: version, executionID: execution.ID, targetResourceID: input.TargetResourceID, providerResourceID: resolved.Provider.ResourceID, modelName: resolved.Model.Name, resourceFilter: resourceFilter, calls: input.MaxToolCalls, resolvedContext: input.ResolvedContext, gateway: input.ToolGateway, allowedAgentTools: profileToolSet(input.AgentProfile), persistExecution: persistExecution, skipToolCallStore: !persistExecution}
	tools, err := policy.tools(ctx)
	if err != nil {
		return finish(FinishExecutionInput{Status: "failed", ErrorCode: "tool_setup", ErrorMessage: publicError(err)})
	}
	instruction := version.Manifest.Instruction
	if input.AgentProfile != nil && strings.TrimSpace(input.AgentProfile.Instruction) != "" {
		if version.SkillResourceID == "" {
			instruction = input.AgentProfile.Instruction
		} else {
			instruction = input.AgentProfile.Instruction + "\n\n" + instruction
		}
	}
	agentConfig := llmagent.Config{Name: "skill_runner", Model: modelClient, Instruction: instruction, Tools: tools, BeforeToolCallbacks: []llmagent.BeforeToolCallback{policy.beforeTool}, DisallowTransferToParent: true, DisallowTransferToPeers: true}
	agentRoot, err := llmagent.New(agentConfig)
	if err != nil {
		return finish(FinishExecutionInput{Status: "failed", ErrorCode: "agent_setup", ErrorMessage: publicError(err)})
	}
	adkRunner, err := runner.NewInMemory(r.AppName, agentRoot)
	if err != nil {
		return finish(FinishExecutionInput{Status: "failed", ErrorCode: "runner_setup", ErrorMessage: publicError(err)})
	}
	messageBytes, _ := json.Marshal(input.Input)
	message := genai.NewContentFromText(string(messageBytes), genai.RoleUser)
	var output strings.Builder
	var promptTokens, completionTokens, totalTokens int64
	var events, iterations int
	for event, eventErr := range adkRunner.Run(ctx, input.ActorID, execution.ID, message, agent.RunConfig{StreamingMode: streamingMode(input.Stream)}) {
		if eventErr != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(eventErr, context.DeadlineExceeded) {
				return finish(FinishExecutionInput{Status: "cancelled", ErrorCode: "timeout", ErrorMessage: "execution timed out", ToolCallCount: policy.used()})
			}
			if errors.Is(ctx.Err(), context.Canceled) || errors.Is(eventErr, context.Canceled) {
				return finish(FinishExecutionInput{Status: "cancelled", ErrorCode: "cancelled", ErrorMessage: "execution cancelled", ToolCallCount: policy.used()})
			}
			return finish(FinishExecutionInput{Status: "failed", ErrorCode: "runner", ErrorMessage: publicError(eventErr), ToolCallCount: policy.used()})
		}
		if event == nil {
			continue
		}
		events++
		if event.Content != nil && countsModelIteration(event.Partial, event.Content.Role) {
			iterations++
			if iterations > input.MaxIterations {
				return finish(FinishExecutionInput{Status: "failed", ErrorCode: "iteration_budget", ErrorMessage: "iteration budget exceeded", PromptTokens: promptTokens, CompletionTokens: completionTokens, TotalTokens: totalTokens, ToolCallCount: policy.used()})
			}
		}
		if event.UsageMetadata != nil {
			promptTokens += int64(event.UsageMetadata.PromptTokenCount)
			completionTokens += int64(event.UsageMetadata.CandidatesTokenCount)
			totalTokens += int64(event.UsageMetadata.TotalTokenCount)
		}
		if event.Content != nil && !event.Partial {
			for _, part := range event.Content.Parts {
				if part != nil && part.Text != "" {
					output.WriteString(part.Text)
				}
			}
		}
		if totalTokens > input.MaxTokens {
			return finish(FinishExecutionInput{Status: "failed", ErrorCode: "token_budget", ErrorMessage: "token budget exceeded", PromptTokens: promptTokens, CompletionTokens: completionTokens, TotalTokens: totalTokens, ToolCallCount: policy.used()})
		}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return finish(FinishExecutionInput{Status: "cancelled", ErrorCode: "timeout", ErrorMessage: "execution timed out", PromptTokens: promptTokens, CompletionTokens: completionTokens, TotalTokens: totalTokens, ToolCallCount: policy.used()})
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return finish(FinishExecutionInput{Status: "cancelled", ErrorCode: "cancelled", ErrorMessage: "execution cancelled", PromptTokens: promptTokens, CompletionTokens: completionTokens, TotalTokens: totalTokens, ToolCallCount: policy.used()})
	}
	text := strings.TrimSpace(output.String())
	if len([]byte(text)) > input.MaxOutputBytes {
		return finish(FinishExecutionInput{Status: "failed", ErrorCode: "output_budget", ErrorMessage: "output budget exceeded", PromptTokens: promptTokens, CompletionTokens: completionTokens, TotalTokens: totalTokens, ToolCallCount: policy.used()})
	}
	if err := validateJSON(version.OutputSchema, []byte(text)); err != nil {
		return finish(FinishExecutionInput{Status: "failed", ErrorCode: "output_schema", ErrorMessage: publicError(err), PromptTokens: promptTokens, CompletionTokens: completionTokens, TotalTokens: totalTokens, ToolCallCount: policy.used()})
	}
	if input.AgentProfile != nil && len(input.AgentProfile.OutputSchema) > 0 {
		if err := validateJSON(input.AgentProfile.OutputSchema, []byte(text)); err != nil {
			return finish(FinishExecutionInput{Status: "failed", ErrorCode: "agent_output_schema", ErrorMessage: publicError(err), PromptTokens: promptTokens, CompletionTokens: completionTokens, TotalTokens: totalTokens, ToolCallCount: policy.used()})
		}
	}
	result, resultErr := finish(FinishExecutionInput{Status: "succeeded", OutputPreview: safePreview(text, 4000), PromptTokens: promptTokens, CompletionTokens: completionTokens, TotalTokens: totalTokens, ToolCallCount: policy.used()})
	result.Output = text
	result.Events = events
	return result, resultErr
}

func countsModelIteration(partial bool, role string) bool {
	return !partial && role == genai.RoleModel
}

func (r *Runner) resolveVersion(ctx context.Context, input RunInput) (Version, error) {
	if strings.TrimSpace(input.SkillResourceID) != "" || strings.TrimSpace(input.SkillVersionID) != "" {
		return r.Skills.Resolve(ctx, input.ScopeID, input.SkillResourceID, input.SkillVersionID)
	}
	if input.AgentProfile == nil {
		return Version{}, invalid("skill_resource_id or agent_profile_id is required")
	}
	return Version{
		Manifest:    Manifest{Name: input.AgentProfile.Name, Description: input.AgentProfile.Description, Instruction: input.AgentProfile.Instruction, TargetKinds: input.AgentProfile.TargetKinds},
		InputSchema: input.AgentProfile.InputSchema, OutputSchema: input.AgentProfile.OutputSchema,
		Status: "published",
	}, nil
}

func missingCapabilities(actual, required []string) []string {
	missing := make([]string, 0)
	for _, capability := range required {
		capability = strings.TrimSpace(capability)
		if capability == "" || slices.Contains(actual, capability) || slices.Contains(missing, capability) {
			continue
		}
		missing = append(missing, capability)
	}
	return missing
}

func profileToolSet(profile *aiengine.AgentProfile) map[string]struct{} {
	if profile == nil || len(profile.AllowedTools) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(profile.AllowedTools))
	for _, name := range profile.AllowedTools {
		set[strings.TrimSpace(name)] = struct{}{}
	}
	return set
}

func streamingMode(stream bool) agent.StreamingMode {
	if stream {
		return agent.StreamingModeSSE
	}
	return agent.StreamingModeNone
}

func validateJSON(schemaRaw json.RawMessage, value []byte) error {
	if len(schemaRaw) == 0 {
		return nil
	}
	var schemaValue any
	if err := json.Unmarshal(schemaRaw, &schemaValue); err != nil {
		return err
	}
	var data any
	if err := json.Unmarshal(value, &data); err != nil {
		return errors.New("value must be valid JSON")
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("memory://schema.json", schemaValue); err != nil {
		return err
	}
	schema, err := compiler.Compile("memory://schema.json")
	if err != nil {
		return err
	}
	return schema.Validate(data)
}

func publicError(err error) string {
	message := strings.TrimSpace(err.Error())
	if len(message) > 1000 {
		return message[:1000]
	}
	return message
}

var secretPattern = regexp.MustCompile(`(?i)(bearer\s+|token|password|secret|api[_-]?key)[:=\s]+[^\s,;]+`)

func safePreview(value string, max int) string {
	value = secretPattern.ReplaceAllString(value, "$1[REDACTED]")
	if len(value) > max {
		return value[:max]
	}
	return value
}

type policy struct {
	runner                        *Runner
	runCtx                        context.Context
	input                         RunInput
	version                       Version
	executionID, targetResourceID string
	providerResourceID, modelName string
	resourceFilter                authorization.ResourceFilter
	resolvedContext               *aiengine.ResolvedContext
	gateway                       *aiengine.PolicyGateway
	allowedAgentTools             map[string]struct{}
	persistExecution              bool
	skipToolCallStore             bool
	calls                         int
	usedCalls, recordedCalls      atomic.Int64
}

func (p *policy) used() int { return int(p.usedCalls.Load()) }
func (p *policy) beforeTool(_ agent.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
	if len(p.allowedAgentTools) > 0 {
		if _, allowed := p.allowedAgentTools[t.Name()]; !allowed {
			return p.rejectTool(t.Name(), args, "agent_tool_not_allowed", fmt.Errorf("tool %q is not allowed by AgentProfile", t.Name()))
		}
	}
	if p.contextTool(t.Name()) {
		used := p.usedCalls.Add(1)
		if used > int64(p.calls) {
			p.usedCalls.Add(-1)
			return p.rejectTool(t.Name(), args, "tool_budget", ErrBudget)
		}
		// A nil result tells ADK to continue with the actual function tool.
		// Returning args here would make ADK treat the callback result as the
		// tool response and skip MCP invocation entirely.
		return nil, nil
	}
	spec, declared := declaredTool(p.version.Tools, t.Name())
	if !declared {
		return p.rejectTool(t.Name(), args, "tool_not_declared", fmt.Errorf("tool %q is not declared by Skill", t.Name()))
	}
	if len(spec.InputSchema) > 0 {
		encoded, err := json.Marshal(args)
		if err != nil {
			return p.rejectTool(t.Name(), args, "invalid_arguments", invalid("tool arguments must be JSON serializable"))
		}
		if err := validateJSON(spec.InputSchema, encoded); err != nil {
			return p.rejectTool(t.Name(), args, "invalid_arguments", fmt.Errorf("validate tool arguments: %w", err))
		}
	}
	target, _ := args["target_resource_id"].(string)
	if target == "" {
		target = p.targetResourceID
		args["target_resource_id"] = target
	}
	if target == "" {
		return p.rejectTool(t.Name(), args, "target_required", invalid("tool target_resource_id is required"))
	}
	if len(p.resourceFilter.ScopeIDs) > 0 || len(p.resourceFilter.ResourceIDs) > 0 {
		item, err := p.runner.Skills.resources.Get(p.runCtx, target)
		if err != nil {
			return p.rejectTool(t.Name(), args, "target_lookup", err)
		}
		if !p.resourceFilter.Allows(item.ScopeID, item.ID) {
			return p.rejectTool(t.Name(), args, "forbidden", authorization.ErrForbidden)
		}
	}
	used := p.usedCalls.Add(1)
	if used > int64(p.calls) {
		p.usedCalls.Add(-1)
		return p.rejectTool(t.Name(), args, "tool_budget", ErrBudget)
	}
	return nil, nil
}

func (p *policy) rejectTool(name string, args map[string]any, code string, cause error) (map[string]any, error) {
	if p.skipToolCallStore {
		p.recordAuditTiming(name, args, nil, "rejected", cause, time.Now().UTC(), code)
		p.emitToolEvent(name, "tool.failed", aiengine.StatusFailed, map[string]any{"error": publicError(cause)})
		return nil, cause
	}
	if p.runner == nil || p.runner.Executions == nil || p.executionID == "" {
		return nil, cause
	}
	encoded, _ := json.Marshal(args)
	digest := sha256.Sum256(encoded)
	sequence := int(p.recordedCalls.Add(1))
	call, err := p.runner.Executions.StartToolCall(context.Background(), p.executionID, sequence, name, "", hex.EncodeToString(digest[:]))
	if err != nil {
		return nil, fmt.Errorf("record rejected tool call: %w", err)
	}
	if _, err := p.runner.Executions.FinishToolCall(context.Background(), call.ID, "rejected", "", code, publicError(cause)); err != nil {
		return nil, fmt.Errorf("finish rejected tool call: %w", err)
	}
	p.recordAuditTiming(name, args, nil, "rejected", cause, time.Now().UTC(), code)
	p.emitToolEvent(name, "tool.failed", aiengine.StatusFailed, map[string]any{"error": publicError(cause)})
	return nil, cause
}
func declaredTool(tools []ToolSpec, name string) (ToolSpec, bool) {
	for _, item := range tools {
		if item.Name == name {
			return item, true
		}
	}
	return ToolSpec{}, false
}

func (p *policy) contextTool(name string) bool {
	if p.resolvedContext == nil || p.gateway == nil {
		return false
	}
	for _, definition := range p.resolvedContext.Tools {
		if definition.Name == name {
			return true
		}
	}
	return false
}

type k8sArgs struct {
	TargetResourceID string `json:"target_resource_id"`
	Resource         string `json:"resource"`
	Namespace        string `json:"namespace,omitempty"`
	Name             string `json:"name,omitempty"`
	LabelSelector    string `json:"label_selector,omitempty"`
	Limit            int64  `json:"limit,omitempty"`
}
type metricsArgs struct {
	TargetResourceID string    `json:"target_resource_id"`
	Query            string    `json:"query"`
	Start            time.Time `json:"start"`
	End              time.Time `json:"end"`
	StepSeconds      int       `json:"step_seconds"`
}
type logsArgs struct {
	TargetResourceID string    `json:"target_resource_id"`
	Query            string    `json:"query"`
	Start            time.Time `json:"start"`
	End              time.Time `json:"end"`
	Limit            int       `json:"limit"`
}
type tracesArgs struct {
	TargetResourceID string    `json:"target_resource_id"`
	Service          string    `json:"service"`
	Operation        string    `json:"operation,omitempty"`
	Start            time.Time `json:"start"`
	End              time.Time `json:"end"`
	Limit            int       `json:"limit"`
}
type alertsArgs struct {
	TargetResourceID string `json:"target_resource_id"`
	ActiveOnly       bool   `json:"active_only"`
}
type middlewareInspectArgs struct {
	TargetResourceID string `json:"target_resource_id"`
}

func (p *policy) tools(ctx context.Context) ([]tool.Tool, error) {
	if p.runner.Connector == nil && len(p.version.Tools) > 0 {
		return nil, errors.New("connector service is unavailable")
	}
	result := make([]tool.Tool, 0, len(p.version.Tools))
	for _, spec := range p.version.Tools {
		var item tool.Tool
		var err error
		switch spec.Name {
		case "connector_kubernetes_read":
			item, err = functiontool.New(functiontool.Config{Name: spec.Name, Description: spec.Description}, func(_ agent.Context, args k8sArgs) (map[string]any, error) {
				return p.execute(ctx, spec.Name, args.TargetResourceID, args, func() (connector.Evidence, error) {
					return p.runner.Connector.ReadKubernetes(ctx, args.TargetResourceID, connector.KubernetesQuery{Resource: args.Resource, Namespace: args.Namespace, Name: args.Name, LabelSelector: args.LabelSelector, Limit: args.Limit})
				})
			})
		case "connector_metrics_query":
			item, err = functiontool.New(functiontool.Config{Name: spec.Name, Description: spec.Description}, func(_ agent.Context, args metricsArgs) (map[string]any, error) {
				return p.execute(ctx, spec.Name, args.TargetResourceID, args, func() (connector.Evidence, error) {
					return p.runner.Connector.QueryMetrics(ctx, args.TargetResourceID, connector.MetricsQuery{Query: args.Query, Start: args.Start, End: args.End, Step: time.Duration(args.StepSeconds) * time.Second})
				})
			})
		case "connector_logs_query":
			item, err = functiontool.New(functiontool.Config{Name: spec.Name, Description: spec.Description}, func(_ agent.Context, args logsArgs) (map[string]any, error) {
				return p.execute(ctx, spec.Name, args.TargetResourceID, args, func() (connector.Evidence, error) {
					return p.runner.Connector.QueryLogs(ctx, args.TargetResourceID, connector.LogsQuery{Query: args.Query, Start: args.Start, End: args.End, Limit: args.Limit})
				})
			})
		case "connector_traces_query":
			item, err = functiontool.New(functiontool.Config{Name: spec.Name, Description: spec.Description}, func(_ agent.Context, args tracesArgs) (map[string]any, error) {
				return p.execute(ctx, spec.Name, args.TargetResourceID, args, func() (connector.Evidence, error) {
					return p.runner.Connector.QueryTraces(ctx, args.TargetResourceID, connector.TracesQuery{Service: args.Service, Operation: args.Operation, Start: args.Start, End: args.End, Limit: args.Limit})
				})
			})
		case "connector_alerts_get":
			item, err = functiontool.New(functiontool.Config{Name: spec.Name, Description: spec.Description}, func(_ agent.Context, args alertsArgs) (map[string]any, error) {
				return p.execute(ctx, spec.Name, args.TargetResourceID, args, func() (connector.Evidence, error) {
					return p.runner.Connector.GetAlerts(ctx, args.TargetResourceID, connector.AlertsQuery{ActiveOnly: args.ActiveOnly})
				})
			})
		case "connector_postgresql_inspect":
			item, err = functiontool.New(functiontool.Config{Name: spec.Name, Description: spec.Description}, func(_ agent.Context, args middlewareInspectArgs) (map[string]any, error) {
				return p.execute(ctx, spec.Name, args.TargetResourceID, args, func() (connector.Evidence, error) {
					return p.runner.Connector.InspectPostgreSQL(ctx, args.TargetResourceID)
				})
			})
		case "connector_redis_inspect":
			item, err = functiontool.New(functiontool.Config{Name: spec.Name, Description: spec.Description}, func(_ agent.Context, args middlewareInspectArgs) (map[string]any, error) {
				return p.execute(ctx, spec.Name, args.TargetResourceID, args, func() (connector.Evidence, error) { return p.runner.Connector.InspectRedis(ctx, args.TargetResourceID) })
			})
		case "connector_kafka_inspect":
			item, err = functiontool.New(functiontool.Config{Name: spec.Name, Description: spec.Description}, func(_ agent.Context, args middlewareInspectArgs) (map[string]any, error) {
				return p.execute(ctx, spec.Name, args.TargetResourceID, args, func() (connector.Evidence, error) { return p.runner.Connector.InspectKafka(ctx, args.TargetResourceID) })
			})
		default:
			return nil, invalid("Skill declares an unsupported tool")
		}
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	for _, definition := range p.contextDefinitions() {
		definition := definition
		item, err := functiontool.New(functiontool.Config{Name: definition.Name, Description: definition.Description}, func(_ agent.Context, args map[string]any) (map[string]any, error) {
			call := aiengine.ToolCall{ExecutionID: p.executionID, ActorID: p.input.ActorID, ScopeID: p.input.ScopeID, ResourceID: definition.ResourceID, Name: definition.Name, ProviderResourceID: p.providerResourceID, ModelName: p.modelName, Arguments: args, EventSink: p.input.EventSink}
			toolResult, invokeErr := p.gateway.Invoke(ctx, call)
			if invokeErr != nil {
				return nil, invokeErr
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

func (p *policy) contextDefinitions() []aiengine.ToolDefinition {
	if p.resolvedContext == nil || p.gateway == nil {
		return nil
	}
	definitions := make([]aiengine.ToolDefinition, 0, len(p.resolvedContext.Tools))
	for _, definition := range p.resolvedContext.Tools {
		if definition.Name != "" && definition.ResourceID != "" {
			definitions = append(definitions, definition)
		}
	}
	return definitions
}

func evidenceMap(evidence connector.Evidence, err error) (map[string]any, error) {
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(evidence)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	err = json.Unmarshal(encoded, &result)
	return result, err
}

func (p *policy) execute(ctx context.Context, name, targetID string, args any, run func() (connector.Evidence, error)) (map[string]any, error) {
	started := time.Now().UTC()
	encoded, _ := json.Marshal(args)
	digest := sha256.Sum256(encoded)
	sequence := int(p.recordedCalls.Add(1))
	call, err := p.runner.Executions.StartToolCall(ctx, p.executionID, sequence, name, targetID, hex.EncodeToString(digest[:]))
	if err != nil {
		return nil, err
	}
	evidence, runErr := run()
	if runErr != nil {
		_, _ = p.runner.Executions.FinishToolCall(context.Background(), call.ID, "failed", "", "connector", publicError(runErr))
		p.recordAuditTiming(name, args, nil, "failed", runErr, started, "connector")
		p.emitToolEvent(name, "tool.failed", aiengine.StatusFailed, map[string]any{"resource_id": targetID, "error": publicError(runErr)})
		return nil, runErr
	}
	if p.input.EvidenceObserver != nil {
		p.input.EvidenceObserver(ObservedEvidence{ToolName: name, TargetResourceID: targetID, Evidence: evidence})
	}
	result, err := evidenceMap(evidence, nil)
	preview, _ := json.Marshal(result)
	_, finishErr := p.runner.Executions.FinishToolCall(context.Background(), call.ID, "succeeded", safePreview(string(preview), 2000), "", "")
	if finishErr != nil {
		return nil, finishErr
	}
	p.recordAuditTiming(name, args, result, "succeeded", nil, started, "")
	p.emitToolEvent(name, "tool.completed", aiengine.StatusSucceeded, map[string]any{"resource_id": targetID})
	return result, err
}

func (p *policy) recordAudit(name string, args, output any, status string, cause error) {
	p.recordAuditTiming(name, args, output, status, cause, time.Time{}, "")
}

func (p *policy) recordAuditTiming(name string, args, output any, status string, cause error, started time.Time, errorCode string) {
	if p.input.ToolCallAudit == nil {
		return
	}
	arguments := make(map[string]any)
	if encoded, encodeErr := json.Marshal(args); encodeErr == nil {
		_ = json.Unmarshal(encoded, &arguments)
	}
	message := ""
	if cause != nil {
		message = publicError(cause)
	}
	completed := time.Now().UTC()
	if started.IsZero() {
		started = completed
	}
	_ = p.input.ToolCallAudit.RecordToolCall(context.Background(), aiengine.ToolCallRecord{ExecutionID: p.executionID, Sequence: int(p.recordedCalls.Load()), ProviderResourceID: p.providerResourceID, ModelName: p.modelName, ResourceID: p.targetResourceID, ToolName: name, Arguments: arguments, Output: output, Status: statusValue(status), ErrorCode: errorCode, Error: message, StartedAt: started, CompletedAt: completed, DurationMS: completed.Sub(started).Milliseconds()})
}

func statusValue(status string) aiengine.Status {
	if status == "succeeded" {
		return aiengine.StatusSucceeded
	}
	return aiengine.StatusFailed
}

func (p *policy) emitToolEvent(name, eventType string, status aiengine.Status, payload map[string]any) {
	if p.input.EventSink == nil {
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["tool"] = name
	_ = p.input.EventSink(aiengine.Event{ExecutionID: p.executionID, Type: eventType, Status: status, Payload: payload})
}
