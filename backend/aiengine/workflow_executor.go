package aiengine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

type WorkflowNodeResult struct {
	Output map[string]any
}

// WorkflowNodeExecutor is the adapter point for Agent, Skill, Tool and
// Retrieval actions. The callback must use the normal AIEngine/Tool Gateway
// paths; the orchestrator only owns ordering and durable state.
type WorkflowNodeExecutor func(context.Context, WorkflowRun, WorkflowNode) (WorkflowNodeResult, error)

type WorkflowExecutor struct {
	Runs   WorkflowRunStore
	Events EventStore
}

func (e WorkflowExecutor) Execute(ctx context.Context, workflow Workflow, runID string, execute WorkflowNodeExecutor) (WorkflowRun, error) {
	if e.Runs == nil || execute == nil {
		return WorkflowRun{}, errors.New("workflow executor dependencies are unavailable")
	}
	if err := workflow.Validate(); err != nil {
		return WorkflowRun{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	run, err := e.Runs.GetWorkflowRun(ctx, runID)
	if err != nil {
		return WorkflowRun{}, err
	}
	if run.WorkflowID != workflow.ID || run.WorkflowVersion != workflow.Version {
		return WorkflowRun{}, fmt.Errorf("%w: workflow version does not match run", ErrWorkflowInvalid)
	}
	if run.Status == WorkflowRunSucceeded || run.Status == WorkflowRunFailed || run.Status == WorkflowRunCancelled {
		return run, nil
	}
	// Reserve a sequence range per workflow attempt so an approval resume never
	// overwrites events emitted by the previous attempt.
	eventSequence := int64(run.Attempt) * 10000
	emit := func(eventType string, status Status, node WorkflowNode, payload map[string]any) {
		if e.Events == nil {
			return
		}
		eventSequence++
		if payload == nil {
			payload = map[string]any{}
		}
		payload["workflow_id"] = workflow.ID
		if node.ID != "" {
			payload["node_id"], payload["node_name"], payload["node_type"] = node.ID, node.Name, node.Type
		}
		_ = e.Events.AppendEvent(context.Background(), Event{ExecutionID: run.ExecutionID, Sequence: eventSequence, Type: eventType, Status: status, Timestamp: time.Now().UTC(), Payload: payload})
	}
	emit("workflow.started", StatusRunning, WorkflowNode{}, map[string]any{"status": run.Status})
	state := cloneMap(run.State)
	completed := map[string]any{}
	if value, ok := state["completed_nodes"].(map[string]any); ok {
		completed = value
	}
	if run.Status == WorkflowRunPending || run.Status == WorkflowRunWaitingApproval {
		run, err = e.Runs.UpdateWorkflowRun(ctx, run.ID, WorkflowRunPatch{Status: WorkflowRunRunning, Attempt: run.Attempt + 1, State: state})
		if err != nil {
			return WorkflowRun{}, err
		}
		if run.CurrentNodeID != "" && run.Status == WorkflowRunRunning {
			if node, ok := workflowNode(workflow, run.CurrentNodeID); ok && node.Type == WorkflowNodeApproval {
				completed[run.CurrentNodeID] = true
			}
		}
	}
	order, err := workflowOrder(workflow)
	if err != nil {
		return WorkflowRun{}, err
	}
	for _, node := range order {
		if _, ok := completed[node.ID]; ok {
			continue
		}
		if err := ctx.Err(); err != nil {
			emit("workflow.cancelled", StatusCancelled, WorkflowNode{}, map[string]any{"error": err.Error()})
			return e.failOrCancel(ctx, run, WorkflowRunCancelled, "cancelled", err.Error(), state)
		}
		run, err = e.Runs.UpdateWorkflowRun(ctx, run.ID, WorkflowRunPatch{Status: WorkflowRunRunning, CurrentNodeID: node.ID, Attempt: run.Attempt, State: state})
		if err != nil {
			return WorkflowRun{}, err
		}
		if node.Type == WorkflowNodeApproval {
			emit("workflow.waiting_approval", StatusRunning, node, map[string]any{"status": WorkflowRunWaitingApproval})
			return e.Runs.UpdateWorkflowRun(ctx, run.ID, WorkflowRunPatch{Status: WorkflowRunWaitingApproval, CurrentNodeID: node.ID, Attempt: run.Attempt, State: state})
		}
		emit("workflow.node.started", StatusRunning, node, nil)
		maxAttempts := node.Retry.MaxAttempts
		if maxAttempts < 1 {
			maxAttempts = 1
		}
		var result WorkflowNodeResult
		var nodeErr error
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			nodeCtx := ctx
			var cancel context.CancelFunc
			if node.TimeoutSecs > 0 {
				nodeCtx, cancel = context.WithTimeout(ctx, time.Duration(node.TimeoutSecs)*time.Second)
			}
			result, nodeErr = execute(nodeCtx, run, node)
			if cancel != nil {
				cancel()
			}
			if nodeErr == nil {
				break
			}
			if attempt < maxAttempts && node.Retry.BackoffSecs > 0 {
				timer := time.NewTimer(time.Duration(node.Retry.BackoffSecs) * time.Second)
				select {
				case <-timer.C:
				case <-ctx.Done():
					timer.Stop()
					nodeErr = ctx.Err()
				}
			}
		}
		if nodeErr != nil {
			status := WorkflowRunFailed
			code := "node_failed"
			if errors.Is(nodeErr, context.Canceled) || errors.Is(nodeErr, context.DeadlineExceeded) {
				status, code = WorkflowRunCancelled, "cancelled"
			}
			emit("workflow.node.failed", StatusFailed, node, map[string]any{"error_code": code, "error": nodeErr.Error()})
			if status == WorkflowRunCancelled {
				emit("workflow.cancelled", StatusCancelled, node, map[string]any{"error": nodeErr.Error()})
			} else {
				emit("workflow.failed", StatusFailed, node, map[string]any{"error": nodeErr.Error()})
			}
			return e.failOrCancel(ctx, run, status, code, nodeErr.Error(), state)
		}
		emit("workflow.node.completed", StatusSucceeded, node, nil)
		completed[node.ID] = true
		state["completed_nodes"] = completed
		if result.Output != nil {
			if outputs, ok := state["node_outputs"].(map[string]any); ok {
				outputs[node.ID] = redactValue(result.Output)
				state["node_outputs"] = outputs
			} else {
				state["node_outputs"] = map[string]any{node.ID: redactValue(result.Output)}
			}
		}
		if _, err := e.Runs.UpdateWorkflowRun(ctx, run.ID, WorkflowRunPatch{Status: WorkflowRunRunning, CurrentNodeID: node.ID, Attempt: run.Attempt, State: state}); err != nil {
			return WorkflowRun{}, err
		}
	}
	emit("workflow.completed", StatusSucceeded, WorkflowNode{}, nil)
	return e.Runs.UpdateWorkflowRun(ctx, run.ID, WorkflowRunPatch{Status: WorkflowRunSucceeded, CurrentNodeID: "", Attempt: run.Attempt, State: state})
}

func (e WorkflowExecutor) failOrCancel(ctx context.Context, run WorkflowRun, status WorkflowRunStatus, code, message string, state map[string]any) (WorkflowRun, error) {
	if ctx.Err() != nil && status == WorkflowRunFailed {
		status, code = WorkflowRunCancelled, "cancelled"
	}
	return e.Runs.UpdateWorkflowRun(context.Background(), run.ID, WorkflowRunPatch{Status: status, CurrentNodeID: run.CurrentNodeID, Attempt: run.Attempt, State: state, ErrorCode: code, ErrorMessage: message})
}

func workflowNode(workflow Workflow, id string) (WorkflowNode, bool) {
	for _, node := range workflow.Nodes {
		if node.ID == id {
			return node, true
		}
	}
	return WorkflowNode{}, false
}

func workflowOrder(workflow Workflow) ([]WorkflowNode, error) {
	indegree := make(map[string]int, len(workflow.Nodes))
	graph := make(map[string][]string, len(workflow.Nodes))
	nodes := make(map[string]WorkflowNode, len(workflow.Nodes))
	for _, node := range workflow.Nodes {
		indegree[node.ID] = 0
		nodes[node.ID] = node
	}
	for _, edge := range workflow.Edges {
		graph[edge.From] = append(graph[edge.From], edge.To)
		indegree[edge.To]++
	}
	ready := make([]string, 0)
	for id, degree := range indegree {
		if degree == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)
	order := make([]WorkflowNode, 0, len(nodes))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		order = append(order, nodes[id])
		children := append([]string(nil), graph[id]...)
		sort.Strings(children)
		for _, child := range children {
			indegree[child]--
			if indegree[child] == 0 {
				ready = append(ready, child)
			}
		}
		sort.Strings(ready)
	}
	if len(order) != len(nodes) {
		return nil, ErrWorkflowCycle
	}
	return order, nil
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
