package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/authorization"
)

var (
	ErrNotFound = errors.New("discovery not found")
	ErrConflict = errors.New("discovery conflict")
	ErrInvalid  = errors.New("invalid discovery request")
)

type store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) Store { return &store{pool: pool} }

func (s *store) CreateRun(ctx context.Context, clusterID, actorID string) (Run, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO discovery_runs (cluster_resource_id, created_by)
		VALUES ($1::uuid, $2::uuid) RETURNING id::text`, clusterID, nullableID(actorID)).Scan(&id)
	if err != nil {
		return Run{}, mapError(err)
	}
	return s.GetRun(ctx, id)
}

func (s *store) GetRun(ctx context.Context, id string) (Run, error) {
	query, args := visibleRunQuery(`
		SELECT run.id::text, run.cluster_resource_id::text, run.status, run.error_message,
		       run.started_at, run.completed_at, run.item_count, run.imported_count,
		       run.created_by::text, run.created_at, run.updated_at
		  FROM discovery_runs run
		  JOIN resources cluster ON cluster.id = run.cluster_resource_id
		 WHERE run.id = $1::uuid AND cluster.deleted_at IS NULL`, ctx, id)
	var item Run
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&item.ID, &item.ClusterResourceID, &item.Status, &item.ErrorMessage,
		&item.StartedAt, &item.CompletedAt, &item.ItemCount, &item.ImportedCount,
		&item.CreatedBy, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Run{}, mapError(err)
	}
	return item, nil
}

func (s *store) ListRuns(ctx context.Context, clusterID string) ([]Run, error) {
	query, args := visibleRunQuery(`
		SELECT run.id::text, run.cluster_resource_id::text, run.status, run.error_message,
		       run.started_at, run.completed_at, run.item_count, run.imported_count,
		       run.created_by::text, run.created_at, run.updated_at
		  FROM discovery_runs run
		  JOIN resources cluster ON cluster.id = run.cluster_resource_id
		 WHERE run.cluster_resource_id = $1::uuid AND cluster.deleted_at IS NULL`, ctx, clusterID)
	query += " ORDER BY run.created_at DESC, run.id"
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list discovery runs: %w", err)
	}
	defer rows.Close()
	items := make([]Run, 0)
	for rows.Next() {
		var item Run
		if err := rows.Scan(&item.ID, &item.ClusterResourceID, &item.Status, &item.ErrorMessage,
			&item.StartedAt, &item.CompletedAt, &item.ItemCount, &item.ImportedCount,
			&item.CreatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan discovery run: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) SetRunning(ctx context.Context, id string) error {
	command, err := s.pool.Exec(ctx, `UPDATE discovery_runs SET status = 'running', started_at = now(), updated_at = now() WHERE id = $1::uuid AND status = 'queued'`, id)
	if err != nil {
		return mapError(err)
	}
	if command.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *store) ReplaceItems(ctx context.Context, runID string, scanned []ScannedItem) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin discovery items: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "DELETE FROM discovery_items WHERE run_id = $1::uuid", runID); err != nil {
		return fmt.Errorf("clear discovery items: %w", err)
	}
	for _, item := range scanned {
		labels, err := json.Marshal(item.Labels)
		if err != nil {
			return fmt.Errorf("encode discovery labels: %w", err)
		}
		payload, err := json.Marshal(item.Payload)
		if err != nil {
			return fmt.Errorf("encode discovery payload: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO discovery_items (run_id, kind, namespace, name, external_uid, resource_version, labels, payload)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb)
			ON CONFLICT (run_id, kind, external_uid) DO UPDATE SET name = EXCLUDED.name,
			  namespace = EXCLUDED.namespace, resource_version = EXCLUDED.resource_version,
			  labels = EXCLUDED.labels, payload = EXCLUDED.payload, status = 'pending',
			  updated_at = now()`, runID, item.Kind, item.Namespace, item.Name, item.ExternalUID,
			item.ResourceVersion, labels, payload); err != nil {
			return mapError(err)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE discovery_runs SET item_count = $2, updated_at = now() WHERE id = $1::uuid AND status = 'running'`, runID, len(scanned)); err != nil {
		return fmt.Errorf("update discovery item count: %w", err)
	}
	return tx.Commit(ctx)
}

func (s *store) CompleteRun(ctx context.Context, id string) error {
	command, err := s.pool.Exec(ctx, `UPDATE discovery_runs SET status = 'succeeded', completed_at = now(), updated_at = now() WHERE id = $1::uuid AND status = 'running'`, id)
	if err != nil {
		return mapError(err)
	}
	if command.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *store) FailRun(ctx context.Context, id string, runErr error) error {
	message := "discovery failed"
	if runErr != nil {
		message = runErr.Error()
	}
	if len(message) > 2000 {
		message = message[:2000]
	}
	_, err := s.pool.Exec(ctx, `UPDATE discovery_runs SET status = 'failed', error_message = $2, completed_at = now(), updated_at = now() WHERE id = $1::uuid`, id, message)
	return mapError(err)
}

func (s *store) ListItems(ctx context.Context, runID string) ([]Item, error) {
	query, args := visibleItemQuery(`
		SELECT item.id::text, item.run_id::text, item.kind, item.namespace, item.name,
		       item.external_uid, item.resource_version, item.labels, item.payload,
		       item.status, item.imported_project_id::text,
		       item.imported_resource_id::text,
		       item.created_at, item.updated_at
		  FROM discovery_items item
		  JOIN discovery_runs run ON run.id = item.run_id
		  JOIN resources cluster ON cluster.id = run.cluster_resource_id
		 WHERE item.run_id = $1::uuid AND cluster.deleted_at IS NULL`, ctx, runID)
	query += " ORDER BY item.namespace, item.kind, item.name"
	return s.scanItems(ctx, query, args...)
}

func (s *store) scanItems(ctx context.Context, query string, args ...any) ([]Item, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list discovery items: %w", err)
	}
	defer rows.Close()
	items := make([]Item, 0)
	for rows.Next() {
		var item Item
		var labels, payload []byte
		if err := rows.Scan(&item.ID, &item.RunID, &item.Kind, &item.Namespace, &item.Name,
			&item.ExternalUID, &item.ResourceVersion, &labels, &payload,
			&item.Status, &item.ImportedProjectID, &item.ImportedResourceID,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan discovery item: %w", err)
		}
		if err := json.Unmarshal(labels, &item.Labels); err != nil {
			return nil, fmt.Errorf("decode discovery labels: %w", err)
		}
		if err := json.Unmarshal(payload, &item.Payload); err != nil {
			return nil, fmt.Errorf("decode discovery payload: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) MarkImported(ctx context.Context, itemID, resourceID, runID string) error {
	return s.markImported(ctx, itemID, runID, "imported_resource_id", resourceID)
}

func (s *store) MarkProjectMapped(ctx context.Context, itemID, projectID, runID string) error {
	return s.markImported(ctx, itemID, runID, "imported_project_id", projectID)
}

func (s *store) markImported(ctx context.Context, itemID, runID, targetColumn, targetID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin discovery import marker: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	command, err := tx.Exec(ctx, `UPDATE discovery_items SET status = 'imported', `+targetColumn+` = $2::uuid, updated_at = now() WHERE id = $1::uuid AND run_id = $3::uuid`, itemID, targetID, runID)
	if err != nil {
		return mapError(err)
	}
	if command.RowsAffected() != 1 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `UPDATE discovery_runs SET imported_count = (SELECT count(*) FROM discovery_items WHERE run_id = $1::uuid AND status = 'imported'), updated_at = now() WHERE id = $1::uuid`, runID); err != nil {
		return mapError(err)
	}
	return tx.Commit(ctx)
}

func (s *store) MarkIgnored(ctx context.Context, itemID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE discovery_items SET status = 'ignored', updated_at = now() WHERE id = $1::uuid`, itemID)
	return mapError(err)
}

func (s *store) MarkMissing(ctx context.Context, clusterID string, currentUIDs []string) error {
	_, err := s.pool.Exec(ctx, `UPDATE resources SET status = 'unknown', updated_at = now() WHERE source_resource_id = $1 AND kind = 'Application' AND deleted_at IS NULL AND ($2::text[] IS NULL OR external_uid <> ALL($2::text[]))`, clusterID, nullableTextArray(currentUIDs))
	return mapError(err)
}

func (s *store) ValidateProjectParent(ctx context.Context, clusterScopeID, teamScopeID string) error {
	var allowed bool
	err := s.pool.QueryRow(ctx, `
		SELECT CASE
		  WHEN cluster.scope_type = 'platform' THEN team.scope_type = 'team' AND resource_scope_contains(cluster.id, team.id)
		  WHEN cluster.scope_type = 'team' THEN cluster.id = team.id
		  ELSE false
		END
		FROM scopes cluster CROSS JOIN scopes team
		WHERE cluster.id = $1::uuid AND team.id = $2::uuid AND cluster.deleted_at IS NULL AND team.deleted_at IS NULL`, clusterScopeID, teamScopeID).Scan(&allowed)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalid
	}
	if err != nil {
		return fmt.Errorf("validate discovery project parent: %w", err)
	}
	if !allowed {
		return ErrInvalid
	}
	return nil
}

func (s *store) ValidateImportTarget(ctx context.Context, clusterScopeID, targetScopeID string) error {
	var allowed bool
	err := s.pool.QueryRow(ctx, `
		SELECT CASE
		  WHEN cluster.scope_type = 'project' THEN cluster.id = target.id
		  WHEN cluster.scope_type IN ('platform', 'team') THEN target.scope_type = 'project' AND resource_scope_contains(cluster.id, target.id)
		  ELSE false
		END
		FROM scopes cluster CROSS JOIN scopes target
		WHERE cluster.id = $1::uuid AND target.id = $2::uuid AND cluster.deleted_at IS NULL AND target.deleted_at IS NULL`, clusterScopeID, targetScopeID).Scan(&allowed)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalid
	}
	if err != nil {
		return fmt.Errorf("validate discovery import target: %w", err)
	}
	if !allowed {
		return ErrInvalid
	}
	return nil
}

func visibleRunQuery(query string, ctx context.Context, args ...any) (string, []any) {
	resourceFilter, resourceRestricted := authorization.ResourceFilterFromContext(ctx)
	filter, restricted := authorization.ScopeFilterFromContext(ctx)
	if !restricted {
		return query, args
	}
	scopePosition := len(args) + 1
	if resourceRestricted {
		resourcePosition := len(args) + 2
		clause := fmt.Sprintf(`(cluster.id = ANY($%d::uuid[]) OR cluster.scope_id = ANY($%d::uuid[]) OR EXISTS (WITH RECURSIVE ancestors(id) AS (SELECT unnest($%d::uuid[]) UNION SELECT scope.parent_scope_id FROM scopes scope JOIN ancestors ON ancestors.id = scope.id WHERE scope.parent_scope_id IS NOT NULL) SELECT 1 FROM ancestors WHERE ancestors.id = cluster.scope_id))`, resourcePosition, scopePosition, scopePosition)
		return query + " AND " + clause, append(args, filter.ScopeIDs, resourceFilter.ResourceIDs)
	}
	clause := fmt.Sprintf(`(cluster.scope_id = ANY($%d::uuid[]) OR EXISTS (WITH RECURSIVE ancestors(id) AS (SELECT unnest($%d::uuid[]) UNION SELECT scope.parent_scope_id FROM scopes scope JOIN ancestors ON ancestors.id = scope.id WHERE scope.parent_scope_id IS NOT NULL) SELECT 1 FROM ancestors WHERE ancestors.id = cluster.scope_id))`, scopePosition, scopePosition)
	return query + " AND " + clause, append(args, filter.ScopeIDs)
}

func visibleItemQuery(query string, ctx context.Context, args ...any) (string, []any) {
	return visibleRunQuery(query, ctx, args...)
}

func nullableID(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullableTextArray(values []string) any {
	if len(values) == 0 {
		return nil
	}
	return values
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrConflict
		case "23503":
			return ErrNotFound
		case "23514":
			return ErrInvalid
		}
	}
	return err
}
