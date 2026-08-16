package operation

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

var ErrNotFound = errors.New("operation not found")
var ErrConflict = errors.New("operation conflict")

type Store interface {
	Create(context.Context, Request) (Request, error)
	Get(context.Context, string) (Request, error)
	List(context.Context, string, int) ([]Request, error)
	Approve(context.Context, Approval) (Request, error)
	StartExecution(context.Context, string, string, time.Time) (string, error)
	GetExecution(context.Context, string) (Execution, error)
}

type PolicyStore interface {
	CreatePolicy(context.Context, Policy) (Policy, error)
	ListPolicies(context.Context, string) ([]Policy, error)
	MatchPolicy(context.Context, string, string, string) (Policy, error)
}
type store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) Store { return &store{pool} }
func (s *store) Create(ctx context.Context, r Request) (Request, error) {
	params, _ := json.Marshal(r.Parameters)
	dry, _ := json.Marshal(r.DryRun)
	err := s.pool.QueryRow(ctx, `INSERT INTO operation_requests(scope_id,target_resource_id,requested_by,source,operation_name,risk_level,parameters,parameters_hash,impact_summary,rollback_summary,dry_run,idempotency_key,status,expires_at) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,$7::jsonb,$8,$9,$10,$11::jsonb,$12,$13,$14) RETURNING id::text,created_at,updated_at`, r.ScopeID, r.TargetResourceID, r.RequestedBy, r.Source, r.OperationName, r.RiskLevel, string(params), r.ParametersHash, r.ImpactSummary, r.RollbackSummary, string(dry), r.IdempotencyKey, r.Status, r.ExpiresAt).Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt)
	return r, err
}
func (s *store) Get(ctx context.Context, id string) (Request, error) {
	var r Request
	var p, d []byte
	err := s.pool.QueryRow(ctx, `SELECT id::text,scope_id::text,target_resource_id::text,requested_by::text,source,operation_name,risk_level,parameters,parameters_hash,impact_summary,rollback_summary,dry_run,idempotency_key,status,expires_at,created_at,updated_at FROM operation_requests WHERE id=$1::uuid`, id).Scan(&r.ID, &r.ScopeID, &r.TargetResourceID, &r.RequestedBy, &r.Source, &r.OperationName, &r.RiskLevel, &p, &r.ParametersHash, &r.ImpactSummary, &r.RollbackSummary, &d, &r.IdempotencyKey, &r.Status, &r.ExpiresAt, &r.CreatedAt, &r.UpdatedAt)
	if err == pgx.ErrNoRows {
		return Request{}, ErrNotFound
	}
	if err == nil {
		_ = json.Unmarshal(p, &r.Parameters)
		_ = json.Unmarshal(d, &r.DryRun)
	}
	return r, err
}

func (s *store) List(ctx context.Context, scopeID string, limit int) ([]Request, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text,scope_id::text,target_resource_id::text,requested_by::text,source,operation_name,risk_level,parameters,parameters_hash,impact_summary,rollback_summary,dry_run,idempotency_key,status,expires_at,created_at,updated_at FROM operation_requests WHERE scope_id=$1::uuid ORDER BY created_at DESC LIMIT $2`, scopeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Request{}
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	return items, rows.Err()
}
func (s *store) Approve(ctx context.Context, a Approval) (Request, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Request{}, err
	}
	defer tx.Rollback(ctx)
	r, err := getTx(ctx, tx, a.OperationRequestID)
	if err != nil {
		return Request{}, err
	}
	if r.RequestedBy == a.ApproverUserID || r.Status != Pending || a.ParametersHash != r.ParametersHash {
		return Request{}, ErrConflict
	}
	if r.ExpiresAt != nil && !time.Now().Before(*r.ExpiresAt) {
		return Request{}, ErrConflict
	}
	if _, err = tx.Exec(ctx, `INSERT INTO operation_approvals(operation_request_id,approver_user_id,decision,parameters_hash,comment) VALUES($1::uuid,$2::uuid,$3,$4,$5)`, a.OperationRequestID, a.ApproverUserID, a.Decision, a.ParametersHash, a.Comment); err != nil {
		return Request{}, err
	}
	status := Approved
	if a.Decision == "rejected" {
		status = Rejected
	}
	if _, err = tx.Exec(ctx, `UPDATE operation_requests SET status=$2,updated_at=now() WHERE id=$1::uuid`, r.ID, status); err != nil {
		return Request{}, err
	}
	r.Status = status
	return r, tx.Commit(ctx)
}
func (s *store) StartExecution(ctx context.Context, id, key string, now time.Time) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	r, err := getTx(ctx, tx, id)
	if err != nil {
		return "", err
	}
	// A retry of the same request is idempotent: once an execution exists, it
	// is returned without reopening approval or changing its key/result.
	var existing string
	err = tx.QueryRow(ctx, `SELECT id::text FROM operation_executions WHERE operation_request_id=$1::uuid`, id).Scan(&existing)
	if err == nil {
		return existing, tx.Commit(ctx)
	}
	if err != pgx.ErrNoRows {
		return "", err
	}
	if RequiresApproval(r.RiskLevel) {
		var a Approval
		err = tx.QueryRow(ctx, `SELECT id::text,operation_request_id::text,approver_user_id::text,decision,parameters_hash,comment,created_at FROM operation_approvals WHERE operation_request_id=$1::uuid AND decision='approved'`, id).Scan(&a.ID, &a.OperationRequestID, &a.ApproverUserID, &a.Decision, &a.ParametersHash, &a.Comment, &a.CreatedAt)
		if err != nil {
			return "", ErrConflict
		}
		if err = CanExecute(r, a, now); err != nil {
			return "", err
		}
	} else if r.Status != Approved {
		return "", ErrConflict
	}
	var execution string
	dryRun, err := json.Marshal(r.DryRun)
	if err != nil {
		return "", err
	}
	err = tx.QueryRow(ctx, `INSERT INTO operation_executions(operation_request_id,executor,idempotency_key,status,result) VALUES($1::uuid,'kubernetes_job',$2,'queued',jsonb_build_object('dry_run',$3::jsonb)) ON CONFLICT(operation_request_id) DO UPDATE SET updated_at=operation_executions.updated_at RETURNING id::text`, id, key, string(dryRun)).Scan(&execution)
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `UPDATE operation_requests SET status='executing',updated_at=now() WHERE id=$1::uuid`, id)
	if err != nil {
		return "", err
	}
	return execution, tx.Commit(ctx)
}

func (s *store) GetExecution(ctx context.Context, id string) (Execution, error) {
	var item Execution
	var result []byte
	err := s.pool.QueryRow(ctx, `SELECT id::text,operation_request_id::text,executor,idempotency_key,status,result,error_message,started_at,completed_at,created_at,updated_at FROM operation_executions WHERE id=$1::uuid`, id).Scan(&item.ID, &item.OperationRequestID, &item.Executor, &item.IdempotencyKey, &item.Status, &result, &item.ErrorMessage, &item.StartedAt, &item.CompletedAt, &item.CreatedAt, &item.UpdatedAt)
	if err == pgx.ErrNoRows {
		return Execution{}, ErrNotFound
	}
	if err == nil {
		_ = json.Unmarshal(result, &item.Result)
	}
	return item, err
}

func (s *store) CreatePolicy(ctx context.Context, item Policy) (Policy, error) {
	if item.Status == "" {
		item.Status = "active"
	}
	err := s.pool.QueryRow(ctx, `INSERT INTO operation_policies(scope_id,name,target_kinds,operation_names,minimum_risk,approval_required,approver_permission,expires_after_seconds,status) VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id::text,created_at,updated_at`, item.ScopeID, item.Name, item.TargetKinds, item.OperationNames, item.MinimumRisk, item.ApprovalRequired, item.ApproverPermission, item.ExpiresAfterSeconds, item.Status).Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}
func (s *store) ListPolicies(ctx context.Context, scopeID string) ([]Policy, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,scope_id::text,name,target_kinds,operation_names,minimum_risk,approval_required,approver_permission,expires_after_seconds,status,created_at,updated_at FROM operation_policies WHERE scope_id=$1::uuid AND deleted_at IS NULL ORDER BY name`, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Policy{}
	for rows.Next() {
		var item Policy
		if err := rows.Scan(&item.ID, &item.ScopeID, &item.Name, &item.TargetKinds, &item.OperationNames, &item.MinimumRisk, &item.ApprovalRequired, &item.ApproverPermission, &item.ExpiresAfterSeconds, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *store) MatchPolicy(ctx context.Context, scopeID, targetKind, operationName string) (Policy, error) {
	var item Policy
	err := s.pool.QueryRow(ctx, `SELECT id::text,scope_id::text,name,target_kinds,operation_names,minimum_risk,approval_required,approver_permission,expires_after_seconds,status,created_at,updated_at FROM operation_policies WHERE scope_id=$1::uuid AND status='active' AND deleted_at IS NULL AND (cardinality(target_kinds)=0 OR $2=ANY(target_kinds)) AND (cardinality(operation_names)=0 OR $3=ANY(operation_names)) ORDER BY cardinality(target_kinds)+cardinality(operation_names) DESC,created_at ASC LIMIT 1`, scopeID, targetKind, operationName).Scan(&item.ID, &item.ScopeID, &item.Name, &item.TargetKinds, &item.OperationNames, &item.MinimumRisk, &item.ApprovalRequired, &item.ApproverPermission, &item.ExpiresAfterSeconds, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if err == pgx.ErrNoRows {
		return Policy{}, ErrNotFound
	}
	return item, err
}
func getTx(ctx context.Context, tx pgx.Tx, id string) (Request, error) {
	var r Request
	var p, d []byte
	err := tx.QueryRow(ctx, `SELECT id::text,scope_id::text,target_resource_id::text,requested_by::text,source,operation_name,risk_level,parameters,parameters_hash,impact_summary,rollback_summary,dry_run,idempotency_key,status,expires_at,created_at,updated_at FROM operation_requests WHERE id=$1::uuid FOR UPDATE`, id).Scan(&r.ID, &r.ScopeID, &r.TargetResourceID, &r.RequestedBy, &r.Source, &r.OperationName, &r.RiskLevel, &p, &r.ParametersHash, &r.ImpactSummary, &r.RollbackSummary, &d, &r.IdempotencyKey, &r.Status, &r.ExpiresAt, &r.CreatedAt, &r.UpdatedAt)
	if err == pgx.ErrNoRows {
		return Request{}, ErrNotFound
	}
	_ = json.Unmarshal(p, &r.Parameters)
	_ = json.Unmarshal(d, &r.DryRun)
	return r, err
}

type scanner interface{ Scan(...any) error }

func scanRequest(row scanner) (Request, error) {
	var r Request
	var p, d []byte
	err := row.Scan(&r.ID, &r.ScopeID, &r.TargetResourceID, &r.RequestedBy, &r.Source, &r.OperationName, &r.RiskLevel, &p, &r.ParametersHash, &r.ImpactSummary, &r.RollbackSummary, &d, &r.IdempotencyKey, &r.Status, &r.ExpiresAt, &r.CreatedAt, &r.UpdatedAt)
	if err == nil {
		_ = json.Unmarshal(p, &r.Parameters)
		_ = json.Unmarshal(d, &r.DryRun)
	}
	return r, err
}
