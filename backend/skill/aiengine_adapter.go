package skill

import (
	"context"

	"opskeeper/backend/aiengine"
	"opskeeper/backend/llm"
)

// AIEngineAdapter connects the Skill Runner to the shared AIEngine contract.
type AIEngineAdapter struct{ runner *Runner }

func (r *Runner) AIEngineAdapter() aiengine.Runner {
	return AIEngineAdapter{runner: r}
}

func (a AIEngineAdapter) Run(ctx context.Context, request aiengine.Request) (aiengine.Result, error) {
	if a.runner == nil {
		return aiengine.Result{}, aiengine.ErrRunnerUnavailable
	}
	input := request.Input
	if input == nil {
		input = map[string]any{"task": request.Task, "messages": request.Messages}
	}
	input = cloneInput(input)
	input["context"] = request.Context
	if request.ResolvedContext != nil {
		input["resolved_context"] = request.ResolvedContext
	}
	runInput := RunInput{
		ExecutionID: request.ExecutionID, ActorID: request.ActorID, ScopeID: request.ScopeID, TargetResourceID: firstResourceID(request.Context),
		SkillResourceID: request.SkillResourceID, SkillVersionID: request.SkillVersionID,
		AIProviderResourceID: request.AIProviderResourceID, ModelName: request.ModelName, Purpose: request.Purpose,
		AgentProfileID: request.AgentProfileID, AgentProfile: request.ResolvedAgentProfile,
		RequiredCapabilities: request.Requirements.Capabilities,
		Input:                input,
		MaxIterations:        request.Budget.MaxIterations, MaxToolCalls: request.Budget.MaxToolCalls, MaxTokens: request.Budget.MaxTokens,
		MaxOutputBytes: request.Budget.MaxOutputBytes, Timeout: request.Budget.Timeout,
		Stream: request.Stream, EventSink: request.EventSink,
		ResolvedContext: request.ResolvedContext, ToolGateway: request.ToolGateway,
	}
	if request.ToolGateway != nil {
		// The Skill Runner records through the same audit store as the generic
		// Tool Gateway.
		runInput.ToolCallAudit = request.ToolGateway.AuditStore
	}
	result, err := a.runner.Run(ctx, runInput)
	if err != nil {
		return aiengine.Result{}, err
	}
	status := aiengine.StatusSucceeded
	if result.Execution.Status == "failed" {
		status = aiengine.StatusFailed
	} else if result.Execution.Status == "cancelled" {
		status = aiengine.StatusCancelled
	}
	return aiengine.Result{
		ExecutionID: result.Execution.ID, Status: status, Output: result.Output,
		PromptTokens: result.Execution.PromptTokens, CompletionTokens: result.Execution.CompletionTokens,
		TotalTokens: result.Execution.TotalTokens, ToolCallCount: result.Execution.ToolCallCount,
		ErrorCode: result.Execution.ErrorCode, ErrorMessage: result.Execution.ErrorMessage,
	}, nil
}

func purposeFromEngine(purpose aiengine.Purpose) llm.Purpose {
	switch purpose {
	case aiengine.PurposeDiagnosis:
		return llm.PurposeDiagnosis
	case aiengine.PurposeInspection:
		return llm.PurposeInspection
	case aiengine.PurposeWorkflow:
		return llm.PurposeWorkflow
	default:
		return llm.PurposeDefault
	}
}

func cloneInput(input map[string]any) map[string]any {
	cloned := make(map[string]any, len(input)+2)
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func firstResourceID(contextRequest aiengine.ContextRequest) string {
	if len(contextRequest.ResourceIDs) == 0 {
		return ""
	}
	return contextRequest.ResourceIDs[0]
}
