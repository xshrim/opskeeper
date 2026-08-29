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
	SetBinding(context.Context, ScopeProviderBinding, string) (ScopeProviderBinding, error)
	RemoveBinding(context.Context, string, Purpose) error
	ListBindings(context.Context, string) ([]ScopeProviderBinding, error)
	ResolveBinding(context.Context, string, Purpose) (ScopeProviderBinding, error)
}

type store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) Store { return &store{pool: pool} }

func (s *store) SetBinding(ctx context.Context, input ScopeProviderBinding, actorID string) (ScopeProviderBinding, error) {
	var item ScopeProviderBinding
	err := s.pool.QueryRow(ctx, `
		INSERT INTO scope_ai_provider_bindings (scope_id, provider_resource_id, tag, created_by)
		VALUES ($1::uuid, $2::uuid, $3, NULLIF($4, '')::uuid)
		ON CONFLICT (scope_id, tag) DO UPDATE SET
			provider_resource_id = EXCLUDED.provider_resource_id,
			updated_at = now()
		RETURNING scope_id::text, provider_resource_id::text, tag, created_at, updated_at`,
		input.ScopeID, input.ProviderID, input.Tag, actorID).Scan(
		&item.ScopeID, &item.ProviderID, &item.Tag, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return ScopeProviderBinding{}, mapStoreError(err)
	}
	return item, nil
}

func (s *store) RemoveBinding(ctx context.Context, scopeID string, purpose Purpose) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM scope_ai_provider_bindings WHERE scope_id = $1::uuid AND tag = $2`, scopeID, purpose)
	if err != nil {
		return fmt.Errorf("remove provider binding: %w", err)
	}
	return nil
}

func (s *store) ListBindings(ctx context.Context, scopeID string) ([]ScopeProviderBinding, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT scope_id::text, provider_resource_id::text, tag, created_at, updated_at
		FROM scope_ai_provider_bindings WHERE scope_id = $1::uuid ORDER BY tag`, scopeID)
	if err != nil {
		return nil, fmt.Errorf("list provider bindings: %w", err)
	}
	defer rows.Close()
	items := make([]ScopeProviderBinding, 0)
	for rows.Next() {
		var item ScopeProviderBinding
		if err := rows.Scan(&item.ScopeID, &item.ProviderID, &item.Tag, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan provider binding: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) ResolveBinding(ctx context.Context, scopeID string, purpose Purpose) (ScopeProviderBinding, error) {
	var item ScopeProviderBinding
	err := s.pool.QueryRow(ctx, `
		WITH RECURSIVE chain(id, parent_scope_id, depth) AS (
			SELECT id, parent_scope_id, 0 FROM scopes
			 WHERE id = $1::uuid AND deleted_at IS NULL AND status = 'active'
			UNION ALL
			SELECT parent.id, parent.parent_scope_id, chain.depth + 1
			  FROM scopes parent JOIN chain ON parent.id = chain.parent_scope_id
			 WHERE parent.deleted_at IS NULL AND parent.status = 'active'
		)
		SELECT bindings.scope_id::text, bindings.provider_resource_id::text,
		       bindings.tag, bindings.created_at, bindings.updated_at
		  FROM chain JOIN scope_ai_provider_bindings bindings ON bindings.scope_id = chain.id
		 WHERE bindings.tag IN ($2, 'default')
		 ORDER BY chain.depth ASC,
		          CASE WHEN bindings.tag = $2 THEN 0 ELSE 1 END
		 LIMIT 1`, scopeID, purpose).Scan(
		&item.ScopeID, &item.ProviderID, &item.Tag, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ScopeProviderBinding{}, ErrNotFound
	}
	if err != nil {
		return ScopeProviderBinding{}, fmt.Errorf("resolve provider binding: %w", err)
	}
	return item, nil
}

func mapStoreError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503", "23514", "22P02":
			return invalid("provider binding references an invalid scope or provider")
		case "23505":
			return ErrConflict
		}
	}
	return fmt.Errorf("store AIProvider configuration: %w", err)
}
