package aiengine

import (
	"errors"
	"testing"
)

func TestWorkflowValidateAcceptsDAGAndRejectsUnknownEdges(t *testing.T) {
	workflow := Workflow{ID: "wf-1", ScopeID: "scope-1", Name: "diagnose", Version: 1, Nodes: []WorkflowNode{
		{ID: "retrieve", Type: WorkflowNodeRetrieval, Name: "retrieve evidence"},
		{ID: "agent", Type: WorkflowNodeAgent, Name: "analyze", Retry: RetryPolicy{MaxAttempts: 2}},
	}, Edges: []WorkflowEdge{{From: "retrieve", To: "agent"}}}
	if err := workflow.Validate(); err != nil {
		t.Fatalf("valid workflow rejected: %v", err)
	}
	workflow.Edges = append(workflow.Edges, WorkflowEdge{From: "agent", To: "missing"})
	if err := workflow.Validate(); err == nil {
		t.Fatal("unknown edge target accepted")
	}
}

func TestWorkflowValidateRejectsCycles(t *testing.T) {
	workflow := Workflow{ID: "wf-1", ScopeID: "scope-1", Name: "cycle", Version: 1, Nodes: []WorkflowNode{
		{ID: "a", Type: WorkflowNodeTool, Name: "a"}, {ID: "b", Type: WorkflowNodeTool, Name: "b"},
	}, Edges: []WorkflowEdge{{From: "a", To: "b"}, {From: "b", To: "a"}}}
	if err := workflow.Validate(); !errors.Is(err, ErrWorkflowCycle) {
		t.Fatalf("cycle error = %v", err)
	}
}

func TestWorkflowRunTransitionGuardsTerminalStates(t *testing.T) {
	run := WorkflowRun{ID: "run-1", WorkflowID: "wf-1", WorkflowVersion: 1, Status: WorkflowRunPending}
	var err error
	if run, err = run.Transition(WorkflowRunRunning); err != nil {
		t.Fatal(err)
	}
	if run, err = run.Transition(WorkflowRunWaitingApproval); err != nil {
		t.Fatal(err)
	}
	if run, err = run.Transition(WorkflowRunRunning); err != nil {
		t.Fatal(err)
	}
	if run, err = run.Transition(WorkflowRunSucceeded); err != nil {
		t.Fatal(err)
	}
	if _, err = run.Transition(WorkflowRunRunning); err == nil {
		t.Fatal("terminal workflow run was allowed to resume")
	}
}
