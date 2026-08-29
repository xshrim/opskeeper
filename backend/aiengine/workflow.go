package aiengine

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// WorkflowNodeType identifies the bounded operations that can appear in a
// persisted AIEngine workflow. Implementations execute the node through the
// normal AIEngine, Tool Gateway and approval boundaries.
type WorkflowNodeType string

const (
	WorkflowNodeAgent     WorkflowNodeType = "agent"
	WorkflowNodeSkill     WorkflowNodeType = "skill"
	WorkflowNodeTool      WorkflowNodeType = "tool"
	WorkflowNodeRetrieval WorkflowNodeType = "retrieval"
	WorkflowNodeCondition WorkflowNodeType = "condition"
	WorkflowNodeParallel  WorkflowNodeType = "parallel"
	WorkflowNodeApproval  WorkflowNodeType = "approval"
)

func (t WorkflowNodeType) valid() bool {
	switch t {
	case WorkflowNodeAgent, WorkflowNodeSkill, WorkflowNodeTool,
		WorkflowNodeRetrieval, WorkflowNodeCondition, WorkflowNodeParallel,
		WorkflowNodeApproval:
		return true
	default:
		return false
	}
}

// Workflow is a scope-owned, versioned DAG. Nodes are immutable once an
// execution starts; edits create a new version at the resource layer.
type Workflow struct {
	ID          string         `json:"id"`
	ScopeID     string         `json:"scope_id"`
	Name        string         `json:"name"`
	Version     int            `json:"version"`
	Description string         `json:"description,omitempty"`
	Nodes       []WorkflowNode `json:"nodes"`
	Edges       []WorkflowEdge `json:"edges"`
	Enabled     bool           `json:"enabled"`
}

// WorkflowFromResource converts the generic Resource config into the typed
// workflow contract and performs the same validation used before execution.
func WorkflowFromResource(id, scopeID, name string, config map[string]any) (Workflow, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return Workflow{}, fmt.Errorf("%w: workflow config is not JSON", ErrWorkflowInvalid)
	}
	var workflow Workflow
	if err := json.Unmarshal(raw, &workflow); err != nil {
		return Workflow{}, fmt.Errorf("%w: workflow config is invalid", ErrWorkflowInvalid)
	}
	workflow.ID, workflow.ScopeID, workflow.Name = strings.TrimSpace(id), strings.TrimSpace(scopeID), strings.TrimSpace(name)
	if err := workflow.Validate(); err != nil {
		return Workflow{}, err
	}
	return workflow, nil
}

// ValidateWorkflowConfig validates a resource config before it has resource
// metadata available. Resource services use this to reject cyclic graphs at
// write time instead of waiting until the first run.
func ValidateWorkflowConfig(config map[string]any) error {
	workflow, err := WorkflowFromResource("workflow", "scope", "workflow", config)
	if err != nil {
		return err
	}
	_ = workflow
	return nil
}

type WorkflowNode struct {
	ID          string           `json:"id"`
	Type        WorkflowNodeType `json:"type"`
	Name        string           `json:"name"`
	Config      map[string]any   `json:"config,omitempty"`
	Retry       RetryPolicy      `json:"retry,omitempty"`
	TimeoutSecs int              `json:"timeout_seconds,omitempty"`
}

type WorkflowEdge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Condition string `json:"condition,omitempty"`
}

type RetryPolicy struct {
	MaxAttempts int `json:"max_attempts,omitempty"`
	BackoffSecs int `json:"backoff_seconds,omitempty"`
}

var (
	ErrWorkflowInvalid = errors.New("workflow is invalid")
	ErrWorkflowCycle   = errors.New("workflow contains a cycle")
)

// Validate checks all structural invariants before a workflow can be stored
// or executed. It deliberately does not interpret node-specific config.
func (w Workflow) Validate() error {
	if strings.TrimSpace(w.ID) == "" || strings.TrimSpace(w.ScopeID) == "" || strings.TrimSpace(w.Name) == "" {
		return fmt.Errorf("%w: id, scope_id and name are required", ErrWorkflowInvalid)
	}
	if w.Version < 1 {
		return fmt.Errorf("%w: version must be positive", ErrWorkflowInvalid)
	}
	if len(w.Nodes) == 0 {
		return fmt.Errorf("%w: at least one node is required", ErrWorkflowInvalid)
	}
	nodes := make(map[string]WorkflowNode, len(w.Nodes))
	for _, node := range w.Nodes {
		id := strings.TrimSpace(node.ID)
		if id == "" || strings.TrimSpace(node.Name) == "" {
			return fmt.Errorf("%w: node id and name are required", ErrWorkflowInvalid)
		}
		if !node.Type.valid() {
			return fmt.Errorf("%w: unsupported node type %q", ErrWorkflowInvalid, node.Type)
		}
		if _, exists := nodes[id]; exists {
			return fmt.Errorf("%w: duplicate node id %q", ErrWorkflowInvalid, id)
		}
		if node.TimeoutSecs < 0 || node.TimeoutSecs > 3600 {
			return fmt.Errorf("%w: node %q timeout is out of range", ErrWorkflowInvalid, id)
		}
		if node.Retry.MaxAttempts < 0 || node.Retry.MaxAttempts > 10 || node.Retry.BackoffSecs < 0 || node.Retry.BackoffSecs > 3600 {
			return fmt.Errorf("%w: node %q retry policy is out of range", ErrWorkflowInvalid, id)
		}
		nodes[id] = node
	}
	adjacency := make(map[string][]string, len(nodes))
	for _, edge := range w.Edges {
		from, to := strings.TrimSpace(edge.From), strings.TrimSpace(edge.To)
		if from == "" || to == "" {
			return fmt.Errorf("%w: edge endpoints are required", ErrWorkflowInvalid)
		}
		if from == to {
			return fmt.Errorf("%w: self-loop at %q", ErrWorkflowInvalid, from)
		}
		if _, ok := nodes[from]; !ok {
			return fmt.Errorf("%w: edge references unknown node %q", ErrWorkflowInvalid, from)
		}
		if _, ok := nodes[to]; !ok {
			return fmt.Errorf("%w: edge references unknown node %q", ErrWorkflowInvalid, to)
		}
		adjacency[from] = append(adjacency[from], to)
	}
	if err := validateAcyclic(adjacency, nodes); err != nil {
		return err
	}
	return nil
}

func validateAcyclic(adjacency map[string][]string, nodes map[string]WorkflowNode) error {
	state := make(map[string]uint8, len(nodes))
	var visit func(string) error
	visit = func(id string) error {
		switch state[id] {
		case 1:
			return fmt.Errorf("%w: node %q", ErrWorkflowCycle, id)
		case 2:
			return nil
		}
		state[id] = 1
		children := append([]string(nil), adjacency[id]...)
		sort.Strings(children)
		for _, child := range children {
			if err := visit(child); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

type WorkflowRunStatus string

const (
	WorkflowRunPending         WorkflowRunStatus = "pending"
	WorkflowRunRunning         WorkflowRunStatus = "running"
	WorkflowRunWaitingApproval WorkflowRunStatus = "waiting_approval"
	WorkflowRunSucceeded       WorkflowRunStatus = "succeeded"
	WorkflowRunFailed          WorkflowRunStatus = "failed"
	WorkflowRunCancelled       WorkflowRunStatus = "cancelled"
)

func validWorkflowTransition(from, to WorkflowRunStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case WorkflowRunPending:
		return to == WorkflowRunRunning || to == WorkflowRunCancelled
	case WorkflowRunRunning:
		return to == WorkflowRunWaitingApproval || to == WorkflowRunSucceeded || to == WorkflowRunFailed || to == WorkflowRunCancelled
	case WorkflowRunWaitingApproval:
		return to == WorkflowRunRunning || to == WorkflowRunFailed || to == WorkflowRunCancelled
	default:
		return false
	}
}

// WorkflowRun tracks resumable execution state. State transitions are kept
// explicit so persistence layers can reject stale or invalid updates.
type WorkflowRun struct {
	ID              string            `json:"id"`
	WorkflowID      string            `json:"workflow_id"`
	WorkflowVersion int               `json:"workflow_version"`
	ExecutionID     string            `json:"execution_id"`
	ScopeID         string            `json:"scope_id"`
	CreatedBy       string            `json:"created_by,omitempty"`
	Status          WorkflowRunStatus `json:"status"`
	CurrentNodeID   string            `json:"current_node_id,omitempty"`
	Attempt         int               `json:"attempt"`
	ErrorCode       string            `json:"error_code,omitempty"`
	ErrorMessage    string            `json:"error_message,omitempty"`
	Input           map[string]any    `json:"input,omitempty"`
	State           map[string]any    `json:"state,omitempty"`
	CreatedAt       time.Time         `json:"created_at,omitempty"`
	UpdatedAt       time.Time         `json:"updated_at,omitempty"`
	CompletedAt     *time.Time        `json:"completed_at,omitempty"`
}

func (r WorkflowRun) Transition(to WorkflowRunStatus) (WorkflowRun, error) {
	if !validWorkflowTransition(r.Status, to) {
		return r, fmt.Errorf("%w: cannot transition from %s to %s", ErrWorkflowInvalid, r.Status, to)
	}
	r.Status = to
	return r, nil
}
