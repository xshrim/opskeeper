package aiengine

import (
	"context"
	"errors"
	"testing"
)

type memoryWorkflowRuns struct{ runs map[string]WorkflowRun }

func (m *memoryWorkflowRuns) CreateWorkflowRun(context.Context, WorkflowRunInput) (WorkflowRun, error) {
	panic("unused")
}
func (m *memoryWorkflowRuns) GetWorkflowRun(_ context.Context, id string) (WorkflowRun, error) {
	run, ok := m.runs[id]
	if !ok {
		return WorkflowRun{}, ErrWorkflowRunNotFound
	}
	return run, nil
}
func (m *memoryWorkflowRuns) ListWorkflowRuns(context.Context, string, int) ([]WorkflowRun, error) {
	panic("unused")
}
func (m *memoryWorkflowRuns) UpdateWorkflowRun(_ context.Context, id string, patch WorkflowRunPatch) (WorkflowRun, error) {
	run, ok := m.runs[id]
	if !ok {
		return WorkflowRun{}, ErrWorkflowRunNotFound
	}
	updated, err := run.Transition(patch.Status)
	if err != nil {
		return WorkflowRun{}, err
	}
	if patch.CurrentNodeID != "" {
		updated.CurrentNodeID = patch.CurrentNodeID
	}
	updated.Attempt = patch.Attempt
	if patch.State != nil {
		updated.State = patch.State
	}
	updated.ErrorCode, updated.ErrorMessage = patch.ErrorCode, patch.ErrorMessage
	m.runs[id] = updated
	return updated, nil
}

func TestWorkflowExecutorPersistsNodeProgressAndResumesApproval(t *testing.T) {
	workflow := Workflow{ID: "wf-1", ScopeID: "scope-1", Name: "test", Version: 1, Enabled: true, Nodes: []WorkflowNode{
		{ID: "a", Type: WorkflowNodeTool, Name: "first"}, {ID: "approval", Type: WorkflowNodeApproval, Name: "approve"}, {ID: "b", Type: WorkflowNodeAgent, Name: "last"},
	}, Edges: []WorkflowEdge{{From: "a", To: "approval"}, {From: "approval", To: "b"}}}
	store := &memoryWorkflowRuns{runs: map[string]WorkflowRun{"run-1": {ID: "run-1", WorkflowID: "wf-1", WorkflowVersion: 1, Status: WorkflowRunPending, State: map[string]any{}}}}
	executor := WorkflowExecutor{Runs: store}
	called := make([]string, 0)
	step := func(_ context.Context, _ WorkflowRun, node WorkflowNode) (WorkflowNodeResult, error) {
		called = append(called, node.ID)
		return WorkflowNodeResult{Output: map[string]any{"secret": "value"}}, nil
	}
	run, err := executor.Execute(context.Background(), workflow, "run-1", step)
	if err != nil || run.Status != WorkflowRunWaitingApproval || len(called) != 1 || called[0] != "a" {
		t.Fatalf("first execution run=%#v called=%v err=%v", run, called, err)
	}
	if _, ok := run.State["node_outputs"].(map[string]any); !ok {
		t.Fatalf("node output snapshot missing: %#v", run.State)
	}
	run, err = executor.Execute(context.Background(), workflow, "run-1", step)
	if err != nil || run.Status != WorkflowRunSucceeded || len(called) != 2 || called[1] != "b" {
		t.Fatalf("resumed execution run=%#v called=%v err=%v", run, called, err)
	}
}

func TestWorkflowExecutorStopsAfterNodeFailure(t *testing.T) {
	workflow := Workflow{ID: "wf-1", ScopeID: "scope-1", Name: "test", Version: 1, Enabled: true, Nodes: []WorkflowNode{{ID: "a", Type: WorkflowNodeTool, Name: "first"}}}
	store := &memoryWorkflowRuns{runs: map[string]WorkflowRun{"run-1": {ID: "run-1", WorkflowID: "wf-1", WorkflowVersion: 1, Status: WorkflowRunPending, State: map[string]any{}}}}
	_, err := (WorkflowExecutor{Runs: store}).Execute(context.Background(), workflow, "run-1", func(context.Context, WorkflowRun, WorkflowNode) (WorkflowNodeResult, error) {
		return WorkflowNodeResult{}, errors.New("failed")
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.runs["run-1"].Status != WorkflowRunFailed || store.runs["run-1"].ErrorCode != "node_failed" {
		t.Fatalf("failed run=%#v", store.runs["run-1"])
	}
}
