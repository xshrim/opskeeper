package aiengine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// WorkflowService connects the durable workflow state machine to the
// existing AIEngine and Tool Gateway contracts. Node configuration is kept
// intentionally declarative and is never allowed to provide provider URLs or
// credentials directly.
type WorkflowService struct {
	Executor  WorkflowExecutor
	Engine    Engine
	Gateway   *PolicyGateway
	Retriever KnowledgeRetriever
}

func NewWorkflowService(runs WorkflowRunStore, engine Engine, gateway *PolicyGateway, retriever KnowledgeRetriever, events ...EventStore) *WorkflowService {
	var eventStore EventStore
	if len(events) > 0 {
		eventStore = events[0]
	}
	service := &WorkflowService{Executor: WorkflowExecutor{Runs: runs, Events: eventStore}, Engine: engine, Gateway: gateway, Retriever: retriever}
	return service
}

func (s *WorkflowService) Execute(ctx context.Context, workflow Workflow, runID string) (WorkflowRun, error) {
	if s == nil || s.Engine == nil {
		return WorkflowRun{}, fmt.Errorf("workflow AIEngine is unavailable")
	}
	return s.Executor.Execute(ctx, workflow, runID, s.node)
}

func (s *WorkflowService) node(ctx context.Context, run WorkflowRun, node WorkflowNode) (WorkflowNodeResult, error) {
	switch node.Type {
	case WorkflowNodeApproval:
		return WorkflowNodeResult{}, nil
	case WorkflowNodeRetrieval:
		return s.retrieve(ctx, run, node)
	case WorkflowNodeTool:
		return s.tool(ctx, run, node)
	case WorkflowNodeAgent, WorkflowNodeSkill:
		return s.model(ctx, run, node)
	case WorkflowNodeCondition:
		return evaluateCondition(run, node)
	case WorkflowNodeParallel:
		return s.parallel(ctx, run, node)
	default:
		return WorkflowNodeResult{}, fmt.Errorf("unsupported workflow node type %q", node.Type)
	}
}

func (s *WorkflowService) parallel(ctx context.Context, run WorkflowRun, node WorkflowNode) (WorkflowNodeResult, error) {
	var config struct {
		Branches []WorkflowNode `json:"branches"`
	}
	if err := decodeNodeConfig(node.Config, &config); err != nil {
		return WorkflowNodeResult{}, err
	}
	if len(config.Branches) == 0 || len(config.Branches) > 32 {
		return WorkflowNodeResult{}, fmt.Errorf("parallel node requires between 1 and 32 branches")
	}
	parallelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	outputs := make(map[string]any, len(config.Branches))
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup
	for _, branch := range config.Branches {
		branch := branch
		if strings.TrimSpace(branch.ID) == "" {
			return WorkflowNodeResult{}, fmt.Errorf("parallel branch id is required")
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := s.node(parallelCtx, run, branch)
			mu.Lock()
			defer mu.Unlock()
			if err != nil && firstErr == nil {
				firstErr = err
				cancel()
				return
			}
			if err == nil {
				outputs[branch.ID] = result.Output
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return WorkflowNodeResult{}, firstErr
	}
	return WorkflowNodeResult{Output: map[string]any{"branches": outputs}}, nil
}

func (s *WorkflowService) model(ctx context.Context, run WorkflowRun, node WorkflowNode) (WorkflowNodeResult, error) {
	if s.Engine == nil {
		return WorkflowNodeResult{}, fmt.Errorf("workflow AIEngine is unavailable")
	}
	var config workflowModelConfig
	if err := decodeNodeConfig(node.Config, &config); err != nil {
		return WorkflowNodeResult{}, err
	}
	input := cloneMap(run.Input)
	for key, value := range config.Input {
		input[key] = value
	}
	request := Request{
		ExecutionID:          config.ExecutionID,
		ActorID:              run.CreatedBy,
		ScopeID:              run.ScopeID,
		Purpose:              config.Purpose,
		AIProviderResourceID: config.AIProviderResourceID,
		ModelName:            config.ModelName,
		Profile:              config.Profile,
		Task:                 config.Task,
		Messages:             config.Messages,
		Input:                input,
		Context:              config.Context,
		SkillResourceID:      config.SkillResourceID,
		SkillVersionID:       config.SkillVersionID,
		AgentProfileID:       config.AgentProfileID,
		Requirements:         config.Requirements,
		Budget:               config.Budget,
		Stream:               config.Stream,
	}
	if request.ExecutionID == "" {
		request.ExecutionID = run.ExecutionID + ":" + node.ID
	}
	result, err := s.Engine.Execute(ctx, request)
	if err != nil {
		return WorkflowNodeResult{}, err
	}
	return WorkflowNodeResult{Output: map[string]any{"execution_id": result.ExecutionID, "status": result.Status, "output": result.Output, "error_code": result.ErrorCode, "error_message": result.ErrorMessage, "tool_call_count": result.ToolCallCount, "total_tokens": result.TotalTokens}}, nil
}

func (s *WorkflowService) tool(ctx context.Context, run WorkflowRun, node WorkflowNode) (WorkflowNodeResult, error) {
	if s.Gateway == nil {
		return WorkflowNodeResult{}, fmt.Errorf("workflow tool gateway is unavailable")
	}
	var config workflowToolConfig
	if err := decodeNodeConfig(node.Config, &config); err != nil {
		return WorkflowNodeResult{}, err
	}
	arguments := cloneMap(run.Input)
	for key, value := range config.Arguments {
		arguments[key] = value
	}
	result, err := s.Gateway.Invoke(ctx, ToolCall{ExecutionID: run.ExecutionID, Sequence: run.Attempt, ScopeID: run.ScopeID, ResourceID: config.ResourceID, Name: config.Name, ProviderResourceID: config.AIProviderResourceID, ModelName: config.ModelName, Arguments: arguments})
	if err != nil {
		return WorkflowNodeResult{}, err
	}
	return WorkflowNodeResult{Output: map[string]any{"output": result.Output, "untrusted": result.Untrusted}}, nil
}

func (s *WorkflowService) retrieve(ctx context.Context, run WorkflowRun, node WorkflowNode) (WorkflowNodeResult, error) {
	if s.Retriever == nil {
		return WorkflowNodeResult{}, fmt.Errorf("workflow knowledge retriever is unavailable")
	}
	var config workflowRetrievalConfig
	if err := decodeNodeConfig(node.Config, &config); err != nil {
		return WorkflowNodeResult{}, err
	}
	result, err := s.Retriever.Retrieve(ctx, KnowledgeQuery{ScopeID: run.ScopeID, Query: config.Query, KnowledgeBaseID: config.KnowledgeBaseID, ResourceIDs: config.ResourceIDs, TopK: config.TopK})
	if err != nil {
		return WorkflowNodeResult{}, err
	}
	return WorkflowNodeResult{Output: map[string]any{"chunks": result.Chunks, "citations": result.Citations, "retrieved_at": result.RetrievedAt}}, nil
}

type workflowModelConfig struct {
	ExecutionID          string         `json:"execution_id,omitempty"`
	Purpose              Purpose        `json:"purpose,omitempty"`
	AIProviderResourceID string         `json:"ai_provider_resource_id,omitempty"`
	ModelName            string         `json:"model_name,omitempty"`
	Profile              Profile        `json:"profile,omitempty"`
	Task                 string         `json:"task,omitempty"`
	Messages             []Message      `json:"messages,omitempty"`
	Input                map[string]any `json:"input,omitempty"`
	Context              ContextRequest `json:"context,omitempty"`
	SkillResourceID      string         `json:"skill_resource_id,omitempty"`
	SkillVersionID       string         `json:"skill_version_id,omitempty"`
	AgentProfileID       string         `json:"agent_profile_id,omitempty"`
	Requirements         Requirements   `json:"requirements,omitempty"`
	Budget               Budget         `json:"budget,omitempty"`
	Stream               bool           `json:"stream,omitempty"`
}

type workflowToolConfig struct {
	ResourceID           string         `json:"resource_id"`
	Name                 string         `json:"name"`
	Arguments            map[string]any `json:"arguments,omitempty"`
	AIProviderResourceID string         `json:"ai_provider_resource_id,omitempty"`
	ModelName            string         `json:"model_name,omitempty"`
}

type workflowRetrievalConfig struct {
	KnowledgeBaseID string   `json:"knowledge_base_id"`
	Query           string   `json:"query"`
	ResourceIDs     []string `json:"resource_ids,omitempty"`
	TopK            int      `json:"top_k,omitempty"`
}

func decodeNodeConfig(config map[string]any, target any) error {
	raw, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("workflow node config is not JSON: %w", err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("workflow node config is invalid: %w", err)
	}
	return nil
}

func evaluateCondition(run WorkflowRun, node WorkflowNode) (WorkflowNodeResult, error) {
	var config struct {
		Key    string `json:"key"`
		Equals any    `json:"equals"`
	}
	if err := decodeNodeConfig(node.Config, &config); err != nil {
		return WorkflowNodeResult{}, err
	}
	if strings.TrimSpace(config.Key) == "" {
		return WorkflowNodeResult{}, fmt.Errorf("condition key is required")
	}
	value, ok := run.State[config.Key]
	matched := ok && fmt.Sprint(value) == fmt.Sprint(config.Equals)
	return WorkflowNodeResult{Output: map[string]any{"matched": matched}}, nil
}
