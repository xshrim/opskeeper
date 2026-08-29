package skill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store interface {
	CreateVersion(context.Context, CreateVersionInput) (Version, error)
	GetVersion(context.Context, string) (Version, error)
	ListVersions(context.Context, string) ([]Version, error)
	PublishVersion(context.Context, string, string) (Version, error)
	DisableVersion(context.Context, string, string) (Version, error)
	SetDefault(context.Context, Default, string) (Default, error)
	ResolveDefault(context.Context, string) (Default, error)
}

type store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) Store { return &store{pool: pool} }

func (s *store) CreateVersion(ctx context.Context, input CreateVersionInput) (Version, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Version{}, fmt.Errorf("begin Skill version transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, input.SkillResourceID); err != nil {
		return Version{}, fmt.Errorf("lock Skill versions: %w", err)
	}
	manifest, _ := json.Marshal(input.Manifest)
	tools, _ := json.Marshal(input.Tools)
	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO skill_versions (skill_resource_id, version, manifest, input_schema, output_schema, tools, risk_level, created_by)
		SELECT $1::uuid, COALESCE(max(version), 0) + 1, $2, $3, $4, $5, $6, NULLIF($7, '')::uuid
		  FROM skill_versions WHERE skill_resource_id = $1::uuid
		RETURNING id::text`, input.SkillResourceID, manifest, input.InputSchema, input.OutputSchema, tools, input.RiskLevel, input.CreatedBy).Scan(&id)
	if err != nil {
		return Version{}, mapStoreError(err)
	}
	item, err := getVersion(ctx, tx, id)
	if err != nil {
		return Version{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Version{}, fmt.Errorf("commit Skill version: %w", err)
	}
	return item, nil
}

func (s *store) GetVersion(ctx context.Context, id string) (Version, error) {
	return getVersion(ctx, s.pool, id)
}

func (s *store) ListVersions(ctx context.Context, skillID string) ([]Version, error) {
	rows, err := s.pool.Query(ctx, versionSelect+` WHERE skill_resource_id = $1::uuid ORDER BY version DESC`, skillID)
	if err != nil {
		return nil, fmt.Errorf("list Skill versions: %w", err)
	}
	defer rows.Close()
	items := make([]Version, 0)
	for rows.Next() {
		item, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) PublishVersion(ctx context.Context, skillID, versionID string) (Version, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Version{}, fmt.Errorf("begin Skill publish: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, skillID); err != nil {
		return Version{}, err
	}
	tag, err := tx.Exec(ctx, `UPDATE skill_versions SET status = 'published', published_at = now() WHERE id = $1::uuid AND skill_resource_id = $2::uuid AND status IN ('draft', 'disabled')`, versionID, skillID)
	if err != nil {
		return Version{}, mapStoreError(err)
	}
	if tag.RowsAffected() == 0 {
		return Version{}, ErrNotFound
	}
	item, err := getVersion(ctx, tx, versionID)
	if err != nil {
		return Version{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Version{}, fmt.Errorf("commit Skill publish: %w", err)
	}
	return item, nil
}

func (s *store) DisableVersion(ctx context.Context, skillID, versionID string) (Version, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE skill_versions SET status = 'disabled' WHERE id = $1::uuid AND skill_resource_id = $2::uuid AND status <> 'disabled'`, versionID, skillID)
	if err != nil {
		return Version{}, mapStoreError(err)
	}
	if tag.RowsAffected() == 0 {
		return Version{}, ErrNotFound
	}
	return s.GetVersion(ctx, versionID)
}

func (s *store) SetDefault(ctx context.Context, input Default, actorID string) (Default, error) {
	var item Default
	err := s.pool.QueryRow(ctx, `
		INSERT INTO skill_scope_defaults (scope_id, skill_resource_id, skill_version_id, created_by)
		VALUES ($1::uuid, $2::uuid, $3::uuid, NULLIF($4, '')::uuid)
		ON CONFLICT (scope_id) DO UPDATE SET skill_resource_id = EXCLUDED.skill_resource_id, skill_version_id = EXCLUDED.skill_version_id, created_by = EXCLUDED.created_by, updated_at = now()
		RETURNING scope_id::text, skill_resource_id::text, skill_version_id::text, created_at, updated_at`,
		input.ScopeID, input.SkillResourceID, input.SkillVersionID, actorID).Scan(&item.ScopeID, &item.SkillResourceID, &item.SkillVersionID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Default{}, mapStoreError(err)
	}
	return item, nil
}

func (s *store) ResolveDefault(ctx context.Context, scopeID string) (Default, error) {
	var item Default
	err := s.pool.QueryRow(ctx, `
		WITH RECURSIVE chain(id, parent_scope_id, depth) AS (
			SELECT id, parent_scope_id, 0 FROM scopes WHERE id = $1::uuid AND deleted_at IS NULL AND status = 'active'
			UNION ALL SELECT parent.id, parent.parent_scope_id, chain.depth + 1 FROM scopes parent JOIN chain ON parent.id = chain.parent_scope_id WHERE parent.deleted_at IS NULL AND parent.status = 'active'
		)
		SELECT defaults.scope_id::text, defaults.skill_resource_id::text, defaults.skill_version_id::text, defaults.created_at, defaults.updated_at
		  FROM chain JOIN skill_scope_defaults defaults ON defaults.scope_id = chain.id
		  JOIN skill_versions version ON version.id = defaults.skill_version_id AND version.status = 'published'
		 ORDER BY chain.depth, defaults.updated_at DESC LIMIT 1`, scopeID).Scan(&item.ScopeID, &item.SkillResourceID, &item.SkillVersionID, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Default{}, ErrNotFound
	}
	if err != nil {
		return Default{}, fmt.Errorf("resolve default Skill: %w", err)
	}
	return item, nil
}

const versionSelect = `SELECT id::text, skill_resource_id::text, version, manifest, input_schema, output_schema, tools, risk_level, status, created_by::text, created_at, published_at FROM skill_versions`

type rowScanner interface{ Scan(...any) error }

func getVersion(ctx context.Context, queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (Version, error) {
	return scanVersion(queryer.QueryRow(ctx, versionSelect+` WHERE id = $1::uuid`, id))
}

func scanVersion(row rowScanner) (Version, error) {
	var item Version
	var manifest, tools []byte
	if err := row.Scan(&item.ID, &item.SkillResourceID, &item.Version, &manifest, &item.InputSchema, &item.OutputSchema, &tools, &item.RiskLevel, &item.Status, &item.CreatedBy, &item.CreatedAt, &item.PublishedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Version{}, ErrNotFound
		}
		return Version{}, fmt.Errorf("scan Skill version: %w", err)
	}
	if err := json.Unmarshal(manifest, &item.Manifest); err != nil {
		return Version{}, fmt.Errorf("decode Skill manifest: %w", err)
	}
	if err := json.Unmarshal(tools, &item.Tools); err != nil {
		return Version{}, fmt.Errorf("decode Skill tools: %w", err)
	}
	return item, nil
}

func mapStoreError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503", "23514", "22P02":
			return invalid("Skill references invalid or unavailable data")
		case "23505":
			return ErrConflict
		}
	}
	return fmt.Errorf("store Skill data: %w", err)
}
