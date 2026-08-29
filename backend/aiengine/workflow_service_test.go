package aiengine

import (
	"context"
	"sync"
	"testing"
)

type workflowServiceEngine struct {
	mu       sync.Mutex
	requests []Request
}

func (e *workflowServiceEngine) Name() string { return "test" }
func (e *workflowServiceEngine) Execute(_ context.Context, request Request) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.requests = append(e.requests, request)
	return Result{ExecutionID: request.ExecutionID, Status: StatusSucceeded, Output: "ok", TotalTokens: 3}, nil
}
func (e *workflowServiceEngine) Stream(context.Context, Request) (<-chan Event, error) {
	return nil, nil
}
func (e *workflowServiceEngine) Cancel(context.Context, string) error { return nil }

type workflowServiceRetriever struct{}

func (workflowServiceRetriever) Retrieve(context.Context, KnowledgeQuery) (RetrievalResult, error) {
	return RetrievalResult{Chunks: []KnowledgeChunk{{ID: "chunk-1", Content: "evidence"}}}.Normalize(), nil
}

func TestWorkflowServiceExecutesModelAndRetrievalNodes(t *testing.T) {
	store := &memoryWorkflowRuns{runs: map[string]WorkflowRun{"run-1": {
		ID: "run-1", WorkflowID: "wf-1", WorkflowVersion: 1, ExecutionID: "exec-1", ScopeID: "scope-1", Status: WorkflowRunPending,
		Input: map[string]any{"from_run": true}, State: map[string]any{},
	}}}
	engine := &workflowServiceEngine{}
	service := NewWorkflowService(store, engine, nil, workflowServiceRetriever{})
	wf := Workflow{ID: "wf-1", ScopeID: "scope-1", Name: "test", Version: 1, Enabled: true, Nodes: []WorkflowNode{
		{ID: "retrieve", Type: WorkflowNodeRetrieval, Name: "retrieve", Config: map[string]any{"knowledge_base_id": "kb-1", "query": "latency"}},
		{ID: "agent", Type: WorkflowNodeAgent, Name: "agent", Config: map[string]any{"task": "summarize", "purpose": "diagnosis"}},
	}, Edges: []WorkflowEdge{{From: "retrieve", To: "agent"}}}
	if _, err := service.Execute(context.Background(), wf, "run-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(engine.requests) != 1 || engine.requests[0].Task != "summarize" || engine.requests[0].Purpose != PurposeDiagnosis {
		t.Fatalf("unexpected model requests: %+v", engine.requests)
	}
	if store.runs["run-1"].Status != WorkflowRunSucceeded {
		t.Fatalf("status=%s", store.runs["run-1"].Status)
	}
	outputs, ok := store.runs["run-1"].State["node_outputs"].(map[string]any)
	if !ok || outputs["retrieve"] == nil || outputs["agent"] == nil {
		t.Fatalf("missing node outputs: %#v", store.runs["run-1"].State)
	}
}

func TestWorkflowServiceToolNodeUsesGateway(t *testing.T) {
	store := &memoryWorkflowRuns{runs: map[string]WorkflowRun{"run-1": {ID: "run-1", WorkflowID: "wf-1", WorkflowVersion: 1, ExecutionID: "exec-1", ScopeID: "scope-1", Status: WorkflowRunPending, State: map[string]any{}}}}
	registry := NewToolRegistry()
	_ = registry.Register("resource-1", ToolFunc{Def: ToolDefinition{Name: "inspect", Source: "test", ResourceID: "resource-1"}, Fn: func(_ context.Context, args map[string]any) (ToolResult, error) { return ToolResult{Output: args}, nil }})
	service := NewWorkflowService(store, &workflowServiceEngine{}, NewPolicyGateway(registry, nil, 0, 0, 0), nil)
	wf := Workflow{ID: "wf-1", ScopeID: "scope-1", Name: "test", Version: 1, Enabled: true, Nodes: []WorkflowNode{{ID: "tool", Type: WorkflowNodeTool, Name: "inspect", Config: map[string]any{"resource_id": "resource-1", "name": "inspect", "arguments": map[string]any{"key": "value"}}}}}
	if _, err := service.Execute(context.Background(), wf, "run-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if store.runs["run-1"].Status != WorkflowRunSucceeded {
		t.Fatalf("status=%s", store.runs["run-1"].Status)
	}
}

func TestWorkflowServiceParallelBranchesRunConcurrently(t *testing.T) {
	store := &memoryWorkflowRuns{runs: map[string]WorkflowRun{"run-1": {ID: "run-1", WorkflowID: "wf-1", WorkflowVersion: 1, ExecutionID: "exec-1", ScopeID: "scope-1", Status: WorkflowRunPending, State: map[string]any{}}}}
	engine := &workflowServiceEngine{}
	service := NewWorkflowService(store, engine, nil, nil)
	wf := Workflow{ID: "wf-1", ScopeID: "scope-1", Name: "parallel", Version: 1, Enabled: true, Nodes: []WorkflowNode{{
		ID: "fanout", Type: WorkflowNodeParallel, Name: "fanout", Config: map[string]any{"branches": []any{
			map[string]any{"id": "one", "type": "agent", "name": "one", "config": map[string]any{"task": "one"}},
			map[string]any{"id": "two", "type": "agent", "name": "two", "config": map[string]any{"task": "two"}},
		}},
	}}}
	if _, err := service.Execute(context.Background(), wf, "run-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(engine.requests) != 2 {
		t.Fatalf("requests=%d, want 2", len(engine.requests))
	}
}
