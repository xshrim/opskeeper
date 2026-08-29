package aiengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrWorkflowRunNotFound = errors.New("workflow run was not found")
	ErrWorkflowRunConflict = errors.New("workflow run conflicts with existing data")
)

type WorkflowRunInput struct {
	WorkflowID      string
	WorkflowVersion int
	ExecutionID     string
	ScopeID         string
	CreatedBy       string
	Input           map[string]any
}

type WorkflowRunPatch struct {
	Status        WorkflowRunStatus
	CurrentNodeID string
	Attempt       int
	State         map[string]any
	ErrorCode     string
	ErrorMessage  string
}

type WorkflowRunStore interface {
	CreateWorkflowRun(context.Context, WorkflowRunInput) (WorkflowRun, error)
	GetWorkflowRun(context.Context, string) (WorkflowRun, error)
	ListWorkflowRuns(context.Context, string, int) ([]WorkflowRun, error)
	UpdateWorkflowRun(context.Context, string, WorkflowRunPatch) (WorkflowRun, error)
}

type PostgresWorkflowRunStore struct{ pool *pgxpool.Pool }

func NewPostgresWorkflowRunStore(pool *pgxpool.Pool) WorkflowRunStore {
	return &PostgresWorkflowRunStore{pool: pool}
}

const workflowRunSelect = `SELECT id::text, workflow_resource_id::text, workflow_version, execution_id, scope_id::text, COALESCE(created_by::text,''), status, current_node_id, attempt, input, state, error_code, error_message, created_at, updated_at, completed_at FROM ai_workflow_runs`

func (s *PostgresWorkflowRunStore) CreateWorkflowRun(ctx context.Context, input WorkflowRunInput) (WorkflowRun, error) {
	if s == nil || s.pool == nil {
		return WorkflowRun{}, errors.New("workflow run store is unavailable")
	}
	input.WorkflowID, input.ExecutionID, input.ScopeID = strings.TrimSpace(input.WorkflowID), strings.TrimSpace(input.ExecutionID), strings.TrimSpace(input.ScopeID)
	if input.WorkflowID == "" || input.ExecutionID == "" || input.ScopeID == "" || input.WorkflowVersion < 1 {
		return WorkflowRun{}, fmt.Errorf("%w: workflow_id, scope_id, execution_id and positive version are required", ErrWorkflowInvalid)
	}
	encoded, err := json.Marshal(input.Input)
	if err != nil {
		return WorkflowRun{}, fmt.Errorf("%w: workflow input is not JSON", ErrWorkflowInvalid)
	}
	var id string
	err = s.pool.QueryRow(ctx, `INSERT INTO ai_workflow_runs(workflow_resource_id,workflow_version,execution_id,scope_id,input,created_by) SELECT $1::uuid,$2,$3,$4::uuid,$5::jsonb,$6::uuid FROM resources WHERE id=$1::uuid AND scope_id=$4::uuid AND kind='Workflow' AND deleted_at IS NULL RETURNING id::text`, input.WorkflowID, input.WorkflowVersion, input.ExecutionID, input.ScopeID, encoded, nullableUUID(input.CreatedBy)).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return WorkflowRun{}, ErrWorkflowRunConflict
		}
		return WorkflowRun{}, fmt.Errorf("create workflow run: %w", err)
	}
	return s.GetWorkflowRun(ctx, id)
}

func nullableUUID(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func (s *PostgresWorkflowRunStore) GetWorkflowRun(ctx context.Context, id string) (WorkflowRun, error) {
	var run WorkflowRun
	var input, state []byte
	var completed *time.Time
	err := s.pool.QueryRow(ctx, workflowRunSelect+` WHERE id=$1::uuid`, strings.TrimSpace(id)).Scan(&run.ID, &run.WorkflowID, &run.WorkflowVersion, &run.ExecutionID, &run.ScopeID, &run.CreatedBy, &run.Status, &run.CurrentNodeID, &run.Attempt, &input, &state, &run.ErrorCode, &run.ErrorMessage, &run.CreatedAt, &run.UpdatedAt, &completed)
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkflowRun{}, ErrWorkflowRunNotFound
	}
	if err != nil {
		return WorkflowRun{}, fmt.Errorf("get workflow run: %w", err)
	}
	run.CompletedAt = completed
	_ = json.Unmarshal(input, &run.Input)
	_ = json.Unmarshal(state, &run.State)
	return run, nil
}

func (s *PostgresWorkflowRunStore) ListWorkflowRuns(ctx context.Context, workflowID string, limit int) ([]WorkflowRun, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, workflowRunSelect+` WHERE workflow_resource_id=$1::uuid ORDER BY created_at DESC LIMIT $2`, strings.TrimSpace(workflowID), limit)
	if err != nil {
		return nil, fmt.Errorf("list workflow runs: %w", err)
	}
	defer rows.Close()
	items := make([]WorkflowRun, 0)
	for rows.Next() {
		var run WorkflowRun
		var input, state []byte
		if err := rows.Scan(&run.ID, &run.WorkflowID, &run.WorkflowVersion, &run.ExecutionID, &run.ScopeID, &run.CreatedBy, &run.Status, &run.CurrentNodeID, &run.Attempt, &input, &state, &run.ErrorCode, &run.ErrorMessage, &run.CreatedAt, &run.UpdatedAt, &run.CompletedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(input, &run.Input)
		_ = json.Unmarshal(state, &run.State)
		items = append(items, run)
	}
	return items, rows.Err()
}

func (s *PostgresWorkflowRunStore) UpdateWorkflowRun(ctx context.Context, id string, patch WorkflowRunPatch) (WorkflowRun, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return WorkflowRun{}, err
	}
	defer tx.Rollback(ctx)
	var run WorkflowRun
	var input, state []byte
	if err := tx.QueryRow(ctx, workflowRunSelect+` WHERE id=$1::uuid FOR UPDATE`, strings.TrimSpace(id)).Scan(&run.ID, &run.WorkflowID, &run.WorkflowVersion, &run.ExecutionID, &run.ScopeID, &run.CreatedBy, &run.Status, &run.CurrentNodeID, &run.Attempt, &input, &state, &run.ErrorCode, &run.ErrorMessage, &run.CreatedAt, &run.UpdatedAt, &run.CompletedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WorkflowRun{}, ErrWorkflowRunNotFound
		}
		return WorkflowRun{}, err
	}
	updated, err := run.Transition(patch.Status)
	if err != nil {
		return WorkflowRun{}, err
	}
	if patch.CurrentNodeID != "" {
		updated.CurrentNodeID = strings.TrimSpace(patch.CurrentNodeID)
	}
	if patch.Attempt >= 0 {
		updated.Attempt = patch.Attempt
	}
	if patch.State != nil {
		state, err = json.Marshal(patch.State)
		if err != nil {
			return WorkflowRun{}, err
		}
	}
	completed := updated.Status == WorkflowRunSucceeded || updated.Status == WorkflowRunFailed || updated.Status == WorkflowRunCancelled
	_, err = tx.Exec(ctx, `UPDATE ai_workflow_runs SET status=$2,current_node_id=$3,attempt=$4,state=$5::jsonb,error_code=$6,error_message=$7,completed_at=CASE WHEN $8 THEN COALESCE(completed_at,now()) ELSE NULL END WHERE id=$1::uuid`, id, updated.Status, updated.CurrentNodeID, updated.Attempt, stateOrEmpty(state), patch.ErrorCode, patch.ErrorMessage, completed)
	if err != nil {
		return WorkflowRun{}, fmt.Errorf("update workflow run: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return WorkflowRun{}, err
	}
	return s.GetWorkflowRun(ctx, id)
}

func stateOrEmpty(value []byte) []byte {
	if len(value) == 0 {
		return []byte(`{}`)
	}
	return value
}
