package llm

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store interface {
	SetDefault(context.Context, Default, string) (Default, error)
	ResolveDefault(context.Context, string) (Default, error)
}

type store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) Store { return &store{pool: pool} }

func (s *store) SetDefault(ctx context.Context, input Default, actorID string) (Default, error) {
	var item Default
	err := s.pool.QueryRow(ctx, `
		INSERT INTO llm_scope_defaults (scope_id, provider_resource_id, model_name, created_by)
		VALUES ($1::uuid, $2::uuid, $3, NULLIF($4, '')::uuid)
		ON CONFLICT (scope_id) DO UPDATE SET
			provider_resource_id = EXCLUDED.provider_resource_id,
			model_name = EXCLUDED.model_name,
			created_by = EXCLUDED.created_by,
			updated_at = now()
		RETURNING scope_id::text, provider_resource_id::text, model_name, created_at, updated_at`,
		input.ScopeID, input.ProviderResourceID, input.ModelName, actorID).Scan(
		&item.ScopeID, &item.ProviderResourceID, &item.ModelName, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Default{}, mapStoreError(err)
	}
	return item, nil
}

func (s *store) ResolveDefault(ctx context.Context, scopeID string) (Default, error) {
	var item Default
	err := s.pool.QueryRow(ctx, `
		WITH RECURSIVE chain(id, parent_scope_id, depth) AS (
			SELECT id, parent_scope_id, 0 FROM scopes
			 WHERE id = $1::uuid AND deleted_at IS NULL AND status = 'active'
			UNION ALL
			SELECT parent.id, parent.parent_scope_id, chain.depth + 1
			  FROM scopes parent JOIN chain ON parent.id = chain.parent_scope_id
			 WHERE parent.deleted_at IS NULL AND parent.status = 'active'
		)
		SELECT defaults.scope_id::text, defaults.provider_resource_id::text,
		       defaults.model_name, defaults.created_at, defaults.updated_at
		  FROM chain JOIN llm_scope_defaults defaults ON defaults.scope_id = chain.id
		 ORDER BY chain.depth ASC LIMIT 1`, scopeID).Scan(
		&item.ScopeID, &item.ProviderResourceID, &item.ModelName, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Default{}, ErrNotFound
	}
	if err != nil {
		return Default{}, fmt.Errorf("resolve default LLM: %w", err)
	}
	return item, nil
}

func mapStoreError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503", "23514", "22P02":
			return invalid("LLM default references an invalid scope, provider or model")
		case "23505":
			return ErrConflict
		}
	}
	return fmt.Errorf("store LLM configuration: %w", err)
}
