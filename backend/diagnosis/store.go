package diagnosis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store interface {
	Start(context.Context, StartInput) (Session, error)
	Get(context.Context, string) (Session, error)
	List(context.Context, string, int) ([]Session, error)
	Targets(context.Context, string) ([]Target, error)
	AddTarget(context.Context, string, string) (Target, error)
	Messages(context.Context, string, int) ([]Message, error)
	AppendMessage(context.Context, string, AppendMessageInput) (Message, error)
	CreatePlan(context.Context, string, string, []PlanStep) (Plan, error)
	Plan(context.Context, string) (Plan, error)
	UpdateStep(context.Context, string, string, string) (PlanStep, error)
	AppendEvent(context.Context, string, CreateEventInput) (Event, error)
	EventsAfter(context.Context, string, int64, int) ([]Event, error)
	SaveEvidence(context.Context, string, CreateEvidenceInput) (Evidence, error)
	Evidence(context.Context, string) ([]Evidence, error)
	SaveHypothesis(context.Context, Hypothesis) (Hypothesis, error)
	Hypotheses(context.Context, string) ([]Hypothesis, error)
	SaveReport(context.Context, Report) (Report, error)
	Report(context.Context, string) (Report, error)
	// ClaimRun atomically moves a queued session into planning.  Its boolean
	// result is false when another worker already owns an active run.
	ClaimRun(context.Context, string) (Session, bool, error)
	SetStatus(context.Context, string, Status) (Session, error)
	Reopen(context.Context, string) (Session, error)
	Finish(context.Context, string, Status, string, string) (Session, error)
}

type store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) Store { return &store{pool: pool} }

func (s *store) Start(ctx context.Context, input StartInput) (Session, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("begin diagnosis session: %w", err)
	}
	defer tx.Rollback(ctx)
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO diagnosis_sessions (scope_id, actor_user_id, status, title) VALUES ($1::uuid, NULLIF($2, '')::uuid, 'queued', $3) RETURNING id::text`, input.ScopeID, input.ActorUserID, input.Title).Scan(&id)
	if err != nil {
		return Session{}, mapStoreError(err)
	}
	for _, resourceID := range input.TargetResourceIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO diagnosis_targets (session_id, resource_id) VALUES ($1::uuid, $2::uuid)`, id, resourceID); err != nil {
			return Session{}, mapStoreError(err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO diagnosis_messages (session_id, role, content) VALUES ($1::uuid, 'user', $2)`, id, input.Question); err != nil {
		return Session{}, mapStoreError(err)
	}
	payload, _ := json.Marshal(map[string]any{"status": StatusQueued, "target_count": len(input.TargetResourceIDs)})
	if _, err := tx.Exec(ctx, `INSERT INTO diagnosis_events (session_id, event_type, payload) VALUES ($1::uuid, 'session.created', $2)`, id, payload); err != nil {
		return Session{}, mapStoreError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Session{}, fmt.Errorf("commit diagnosis session: %w", err)
	}
	return s.Get(ctx, id)
}

func (s *store) Get(ctx context.Context, id string) (Session, error) {
	return scanSession(s.pool.QueryRow(ctx, sessionSelect+` WHERE id = $1::uuid`, id))
}

func (s *store) List(ctx context.Context, scopeID string, limit int) ([]Session, error) {
	rows, err := s.pool.Query(ctx, sessionSelect+` WHERE scope_id = $1::uuid ORDER BY created_at DESC LIMIT $2`, scopeID, limit)
	if err != nil {
		return nil, fmt.Errorf("list diagnosis sessions: %w", err)
	}
	defer rows.Close()
	items := make([]Session, 0)
	for rows.Next() {
		item, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) Targets(ctx context.Context, sessionID string) ([]Target, error) {
	rows, err := s.pool.Query(ctx, `SELECT session_id::text, resource_id::text, created_at FROM diagnosis_targets WHERE session_id = $1::uuid ORDER BY created_at, resource_id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list diagnosis targets: %w", err)
	}
	defer rows.Close()
	items := make([]Target, 0)
	for rows.Next() {
		var item Target
		if err := rows.Scan(&item.SessionID, &item.ResourceID, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan diagnosis target: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) AddTarget(ctx context.Context, sessionID, resourceID string) (Target, error) {
	var item Target
	err := s.pool.QueryRow(ctx, `INSERT INTO diagnosis_targets (session_id, resource_id) VALUES ($1::uuid, $2::uuid) ON CONFLICT DO NOTHING RETURNING session_id::text, resource_id::text, created_at`, sessionID, resourceID).Scan(&item.SessionID, &item.ResourceID, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Target{SessionID: sessionID, ResourceID: resourceID}, nil
	}
	if err != nil {
		return Target{}, mapStoreError(err)
	}
	return item, nil
}

func (s *store) Messages(ctx context.Context, sessionID string, limit int) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, session_id::text, role, content, created_at FROM diagnosis_messages WHERE session_id = $1::uuid ORDER BY created_at DESC, id DESC LIMIT $2`, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("list diagnosis messages: %w", err)
	}
	defer rows.Close()
	items := make([]Message, 0)
	for rows.Next() {
		var item Message
		if err := rows.Scan(&item.ID, &item.SessionID, &item.Role, &item.Content, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan diagnosis message: %w", err)
		}
		items = append(items, item)
	}
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	return items, rows.Err()
}

func (s *store) AppendMessage(ctx context.Context, sessionID string, input AppendMessageInput) (Message, error) {
	var item Message
	err := s.pool.QueryRow(ctx, `INSERT INTO diagnosis_messages (session_id, role, content) VALUES ($1::uuid, $2, $3) RETURNING id::text, session_id::text, role, content, created_at`, sessionID, input.Role, input.Content).Scan(&item.ID, &item.SessionID, &item.Role, &item.Content, &item.CreatedAt)
	if err != nil {
		return Message{}, mapStoreError(err)
	}
	return item, nil
}

func (s *store) CreatePlan(ctx context.Context, sessionID, summary string, steps []PlanStep) (Plan, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Plan{}, fmt.Errorf("begin diagnosis plan: %w", err)
	}
	defer tx.Rollback(ctx)
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO diagnosis_plans (session_id, summary) VALUES ($1::uuid, $2) ON CONFLICT (session_id) DO UPDATE SET summary = EXCLUDED.summary, updated_at = now() RETURNING id::text`, sessionID, summary).Scan(&id); err != nil {
		return Plan{}, mapStoreError(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM diagnosis_plan_steps WHERE plan_id = $1::uuid`, id); err != nil {
		return Plan{}, mapStoreError(err)
	}
	for index, step := range steps {
		if _, err := tx.Exec(ctx, `INSERT INTO diagnosis_plan_steps (plan_id, sequence, phase, status, title, detail) VALUES ($1::uuid, $2, $3, $4, $5, $6)`, id, index+1, step.Phase, step.Status, step.Title, step.Detail); err != nil {
			return Plan{}, mapStoreError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Plan{}, fmt.Errorf("commit diagnosis plan: %w", err)
	}
	return s.plan(ctx, sessionID)
}

func (s *store) UpdateStep(ctx context.Context, stepID, status, detail string) (PlanStep, error) {
	var item PlanStep
	err := s.pool.QueryRow(ctx, `UPDATE diagnosis_plan_steps SET status = $2, detail = $3, updated_at = now() WHERE id = $1::uuid RETURNING id::text, plan_id::text, sequence, phase, status, title, detail, created_at, updated_at`, stepID, status, detail).Scan(&item.ID, &item.PlanID, &item.Sequence, &item.Phase, &item.Status, &item.Title, &item.Detail, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return PlanStep{}, mapStoreError(err)
	}
	return item, nil
}

func (s *store) AppendEvent(ctx context.Context, sessionID string, input CreateEventInput) (Event, error) {
	payload, err := json.Marshal(input.Payload)
	if err != nil {
		return Event{}, fmt.Errorf("encode diagnosis event: %w", err)
	}
	var item Event
	err = s.pool.QueryRow(ctx, `INSERT INTO diagnosis_events (session_id, event_type, payload) VALUES ($1::uuid, $2, $3) RETURNING id, session_id::text, event_type, payload, created_at`, sessionID, input.Type, payload).Scan(&item.ID, &item.SessionID, &item.Type, &item.Payload, &item.CreatedAt)
	if err != nil {
		return Event{}, mapStoreError(err)
	}
	return item, nil
}

func (s *store) EventsAfter(ctx context.Context, sessionID string, after int64, limit int) ([]Event, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, session_id::text, event_type, payload, created_at FROM diagnosis_events WHERE session_id = $1::uuid AND id > $2 ORDER BY id LIMIT $3`, sessionID, after, limit)
	if err != nil {
		return nil, fmt.Errorf("list diagnosis events: %w", err)
	}
	defer rows.Close()
	items := make([]Event, 0)
	for rows.Next() {
		var item Event
		if err := rows.Scan(&item.ID, &item.SessionID, &item.Type, &item.Payload, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan diagnosis event: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) SaveEvidence(ctx context.Context, sessionID string, input CreateEvidenceInput) (Evidence, error) {
	content := input.Content
	if len(content) == 0 {
		content = json.RawMessage(`{}`)
	}
	summary := input.Summary
	if len(summary) == 0 {
		summary = json.RawMessage(`{}`)
	}
	digest := sha256.Sum256(content)
	var item Evidence
	err := s.pool.QueryRow(ctx, `INSERT INTO diagnosis_evidence (session_id, target_resource_id, source_resource_id, capability, collected_at, window_start, window_end, content_hash, summary, content, partial, untrusted) VALUES ($1::uuid, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id::text, session_id::text, target_resource_id::text, source_resource_id::text, capability, collected_at, window_start, window_end, content_hash, summary, content, partial, untrusted, created_at`, sessionID, input.TargetResourceID, input.SourceResourceID, input.Capability, input.CollectedAt, input.WindowStart, input.WindowEnd, hex.EncodeToString(digest[:]), summary, content, input.Partial, input.Untrusted).Scan(&item.ID, &item.SessionID, &item.TargetResourceID, &item.SourceResourceID, &item.Capability, &item.CollectedAt, &item.WindowStart, &item.WindowEnd, &item.ContentHash, &item.Summary, &item.Content, &item.Partial, &item.Untrusted, &item.CreatedAt)
	if err != nil {
		return Evidence{}, mapStoreError(err)
	}
	return item, nil
}

func (s *store) Evidence(ctx context.Context, sessionID string) ([]Evidence, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, session_id::text, target_resource_id::text, source_resource_id::text, capability, collected_at, window_start, window_end, content_hash, summary, content, partial, untrusted, created_at FROM diagnosis_evidence WHERE session_id = $1::uuid ORDER BY created_at, id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list diagnosis evidence: %w", err)
	}
	defer rows.Close()
	items := make([]Evidence, 0)
	for rows.Next() {
		var item Evidence
		if err := rows.Scan(&item.ID, &item.SessionID, &item.TargetResourceID, &item.SourceResourceID, &item.Capability, &item.CollectedAt, &item.WindowStart, &item.WindowEnd, &item.ContentHash, &item.Summary, &item.Content, &item.Partial, &item.Untrusted, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan diagnosis evidence: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) SaveHypothesis(ctx context.Context, input Hypothesis) (Hypothesis, error) {
	if err := s.validateEvidenceReferences(ctx, input.SessionID, input.EvidenceIDs); err != nil {
		return Hypothesis{}, err
	}
	var item Hypothesis
	err := s.pool.QueryRow(ctx, `INSERT INTO diagnosis_hypotheses (session_id, statement, status, confidence, evidence_ids) VALUES ($1::uuid, $2, $3, $4, $5::uuid[]) RETURNING id::text, session_id::text, statement, status, confidence::float8, evidence_ids::text[], created_at, updated_at`, input.SessionID, input.Statement, input.Status, input.Confidence, input.EvidenceIDs).Scan(&item.ID, &item.SessionID, &item.Statement, &item.Status, &item.Confidence, &item.EvidenceIDs, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Hypothesis{}, mapStoreError(err)
	}
	return item, nil
}

func (s *store) Hypotheses(ctx context.Context, sessionID string) ([]Hypothesis, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, session_id::text, statement, status, confidence::float8, evidence_ids::text[], created_at, updated_at FROM diagnosis_hypotheses WHERE session_id = $1::uuid ORDER BY created_at, id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list diagnosis hypotheses: %w", err)
	}
	defer rows.Close()
	items := make([]Hypothesis, 0)
	for rows.Next() {
		var item Hypothesis
		if err := rows.Scan(&item.ID, &item.SessionID, &item.Statement, &item.Status, &item.Confidence, &item.EvidenceIDs, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan diagnosis hypothesis: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) SaveReport(ctx context.Context, input Report) (Report, error) {
	if err := s.validateEvidenceReferences(ctx, input.SessionID, input.EvidenceIDs); err != nil {
		return Report{}, err
	}
	var item Report
	err := s.pool.QueryRow(ctx, `INSERT INTO diagnosis_reports (session_id, status, conclusion, recommendations, evidence_ids) VALUES ($1::uuid, $2, $3, $4, $5::uuid[]) ON CONFLICT (session_id) DO UPDATE SET status = EXCLUDED.status, conclusion = EXCLUDED.conclusion, recommendations = EXCLUDED.recommendations, evidence_ids = EXCLUDED.evidence_ids RETURNING id::text, session_id::text, status, conclusion, recommendations, evidence_ids::text[], created_at`, input.SessionID, input.Status, input.Conclusion, input.Recommendations, input.EvidenceIDs).Scan(&item.ID, &item.SessionID, &item.Status, &item.Conclusion, &item.Recommendations, &item.EvidenceIDs, &item.CreatedAt)
	if err != nil {
		return Report{}, mapStoreError(err)
	}
	return item, nil
}

func (s *store) validateEvidenceReferences(ctx context.Context, sessionID string, evidenceIDs []string) error {
	if len(evidenceIDs) == 0 {
		return nil
	}
	var count int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM diagnosis_evidence WHERE session_id = $1::uuid AND id = ANY($2::uuid[])`, sessionID, evidenceIDs).Scan(&count)
	if err != nil {
		return mapStoreError(err)
	}
	if count != len(evidenceIDs) {
		return invalid("all referenced Evidence must belong to the diagnosis session")
	}
	return nil
}

func (s *store) Report(ctx context.Context, sessionID string) (Report, error) {
	var item Report
	err := s.pool.QueryRow(ctx, `SELECT id::text, session_id::text, status, conclusion, recommendations, evidence_ids::text[], created_at FROM diagnosis_reports WHERE session_id = $1::uuid`, sessionID).Scan(&item.ID, &item.SessionID, &item.Status, &item.Conclusion, &item.Recommendations, &item.EvidenceIDs, &item.CreatedAt)
	if err != nil {
		return Report{}, mapStoreError(err)
	}
	return item, nil
}

func (s *store) ClaimRun(ctx context.Context, sessionID string) (Session, bool, error) {
	var item Session
	err := s.pool.QueryRow(ctx, `UPDATE diagnosis_sessions
		SET status = 'planning', updated_at = now()
		WHERE id = $1::uuid AND status = 'queued'
		RETURNING id::text, scope_id::text, actor_user_id::text, status, title, error_code, error_message, started_at, completed_at, created_at, updated_at`, sessionID).
		Scan(&item.ID, &item.ScopeID, &item.ActorUserID, &item.Status, &item.Title, &item.ErrorCode, &item.ErrorMessage, &item.StartedAt, &item.CompletedAt, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, mapStoreError(err)
	}
	return item, true, nil
}

func (s *store) Finish(ctx context.Context, sessionID string, status Status, code, message string) (Session, error) {
	if status != StatusSucceeded && status != StatusFailed && status != StatusCancelled {
		return Session{}, invalid("a diagnosis can only finish as succeeded, failed, or cancelled")
	}
	tag, err := s.pool.Exec(ctx, `UPDATE diagnosis_sessions SET status = $2, error_code = $3, error_message = $4, completed_at = now(), updated_at = now() WHERE id = $1::uuid AND status NOT IN ('succeeded', 'failed', 'cancelled')`, sessionID, status, code, message)
	if err != nil {
		return Session{}, mapStoreError(err)
	}
	if tag.RowsAffected() == 0 {
		return Session{}, ErrConflict
	}
	return s.Get(ctx, sessionID)
}

func (s *store) SetStatus(ctx context.Context, sessionID string, status Status) (Session, error) {
	if status != StatusQueued && status != StatusPlanning && status != StatusCollecting && status != StatusAnalyzing {
		return Session{}, invalid("diagnosis status must be an active state")
	}
	tag, err := s.pool.Exec(ctx, `UPDATE diagnosis_sessions SET status = $2, updated_at = now() WHERE id = $1::uuid AND status NOT IN ('succeeded', 'failed', 'cancelled')`, sessionID, status)
	if err != nil {
		return Session{}, mapStoreError(err)
	}
	if tag.RowsAffected() == 0 {
		return Session{}, ErrConflict
	}
	return s.Get(ctx, sessionID)
}

func (s *store) Reopen(ctx context.Context, sessionID string) (Session, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE diagnosis_sessions SET status = 'queued', error_code = '', error_message = '', completed_at = NULL, updated_at = now() WHERE id = $1::uuid AND status IN ('succeeded', 'failed', 'cancelled')`, sessionID)
	if err != nil {
		return Session{}, mapStoreError(err)
	}
	if tag.RowsAffected() == 0 {
		return s.Get(ctx, sessionID)
	}
	return s.Get(ctx, sessionID)
}

func (s *store) plan(ctx context.Context, sessionID string) (Plan, error) {
	var item Plan
	err := s.pool.QueryRow(ctx, `SELECT id::text, session_id::text, summary, created_at, updated_at FROM diagnosis_plans WHERE session_id = $1::uuid`, sessionID).Scan(&item.ID, &item.SessionID, &item.Summary, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Plan{}, mapStoreError(err)
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text, plan_id::text, sequence, phase, status, title, detail, created_at, updated_at FROM diagnosis_plan_steps WHERE plan_id = $1::uuid ORDER BY sequence`, item.ID)
	if err != nil {
		return Plan{}, fmt.Errorf("list diagnosis plan steps: %w", err)
	}
	defer rows.Close()
	item.Steps = make([]PlanStep, 0)
	for rows.Next() {
		var step PlanStep
		if err := rows.Scan(&step.ID, &step.PlanID, &step.Sequence, &step.Phase, &step.Status, &step.Title, &step.Detail, &step.CreatedAt, &step.UpdatedAt); err != nil {
			return Plan{}, fmt.Errorf("scan diagnosis plan step: %w", err)
		}
		item.Steps = append(item.Steps, step)
	}
	if err := rows.Err(); err != nil {
		return Plan{}, err
	}
	return item, nil
}

func (s *store) Plan(ctx context.Context, sessionID string) (Plan, error) {
	return s.plan(ctx, sessionID)
}

const sessionSelect = `SELECT id::text, scope_id::text, actor_user_id::text, status, title, error_code, error_message, started_at, completed_at, created_at, updated_at FROM diagnosis_sessions`

type rowScanner interface{ Scan(...any) error }

func scanSession(row rowScanner) (Session, error) {
	var item Session
	if err := row.Scan(&item.ID, &item.ScopeID, &item.ActorUserID, &item.Status, &item.Title, &item.ErrorCode, &item.ErrorMessage, &item.StartedAt, &item.CompletedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrNotFound
		}
		return Session{}, fmt.Errorf("scan diagnosis session: %w", err)
	}
	return item, nil
}

func mapStoreError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503", "23514", "22P02":
			return invalid("diagnosis references invalid or unavailable data")
		case "23505":
			return ErrConflict
		}
	}
	return fmt.Errorf("store diagnosis data: %w", err)
}
