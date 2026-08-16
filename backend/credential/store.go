package credential

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/authorization"
)

type Store interface {
	Create(context.Context, string, CreateInput, []byte, string) (Credential, error)
	List(context.Context, string) ([]Credential, error)
	Get(context.Context, string, string) (Credential, error)
	Update(context.Context, string, string, UpdateInput, []byte, string) (Credential, error)
	Delete(context.Context, string, string) error
}

type store struct{ pool *pgxpool.Pool }

var _ Store = (*store)(nil)

func NewStore(pool *pgxpool.Pool) Store { return &store{pool: pool} }

const credentialSelect = `
SELECT credential.id::text, credential.scope_id::text, credential.name,
       credential.purpose, credential.key_version, credential.created_at,
       credential.updated_at
  FROM resource_credentials credential
 WHERE credential.deleted_at IS NULL`

func (s *store) Create(ctx context.Context, actorID string, input CreateInput, ciphertext []byte, keyVersion string) (Credential, error) {
	var item Credential
	_, err := s.pool.Exec(ctx, `
        INSERT INTO resource_credentials (scope_id, name, purpose, ciphertext, key_version, created_by)
        VALUES ($1::uuid, $2, $3, $4, $5, $6::uuid)`, input.ScopeID, input.Name, input.Purpose, ciphertext, keyVersion, actorID)
	if err != nil {
		return Credential{}, mapStoreError(err)
	}
	err = s.pool.QueryRow(ctx, credentialSelect+` AND credential.scope_id = $1::uuid AND lower(credential.name) = lower($2)`, input.ScopeID, input.Name).
		Scan(&item.ID, &item.ScopeID, &item.Name, &item.Purpose, &item.KeyVersion, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Credential{}, fmt.Errorf("read created credential: %w", err)
	}
	return item, nil
}

func (s *store) List(ctx context.Context, actorID string) ([]Credential, error) {
	query, args := visibleQuery(credentialSelect, "credential", ctx)
	query += " ORDER BY credential.created_at DESC, credential.id"
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}
	defer rows.Close()
	items := make([]Credential, 0)
	for rows.Next() {
		var item Credential
		if err := rows.Scan(&item.ID, &item.ScopeID, &item.Name, &item.Purpose, &item.KeyVersion, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan credential: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) Get(ctx context.Context, actorID, id string) (Credential, error) {
	query, args := visibleQuery(credentialSelect+" AND credential.id = $1::uuid", "credential", ctx, id)
	var item Credential
	if err := s.pool.QueryRow(ctx, query, args...).Scan(&item.ID, &item.ScopeID, &item.Name, &item.Purpose, &item.KeyVersion, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return Credential{}, mapStoreError(err)
	}
	return item, nil
}

func (s *store) Update(ctx context.Context, actorID, id string, input UpdateInput, ciphertext []byte, keyVersion string) (Credential, error) {
	set := []string{"updated_at = now()"}
	args := []any{id}
	if input.Name != nil {
		args = append(args, *input.Name)
		set = append(set, "name = $"+strconv.Itoa(len(args)))
	}
	if input.Purpose != nil {
		args = append(args, *input.Purpose)
		set = append(set, "purpose = $"+strconv.Itoa(len(args)))
	}
	if ciphertext != nil {
		args = append(args, ciphertext, keyVersion)
		set = append(set, "ciphertext = $"+strconv.Itoa(len(args)-1), "key_version = $"+strconv.Itoa(len(args)))
	}
	query, visibleArgs := exactQuery("UPDATE resource_credentials credential SET "+strings.Join(set, ", ")+" WHERE credential.id = $1::uuid AND credential.deleted_at IS NULL", "credential", ctx, args...)
	command, err := s.pool.Exec(ctx, query, visibleArgs...)
	if err != nil {
		return Credential{}, mapStoreError(err)
	}
	if command.RowsAffected() != 1 {
		return Credential{}, ErrNotFound
	}
	return s.Get(ctx, actorID, id)
}

func (s *store) Delete(ctx context.Context, actorID, id string) error {
	query, args := exactQuery("UPDATE resource_credentials credential SET deleted_at = now(), updated_at = now() WHERE credential.id = $1::uuid AND credential.deleted_at IS NULL", "credential", ctx, id)
	command, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return mapStoreError(err)
	}
	if command.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

func visibleQuery(query, alias string, ctx context.Context, args ...any) (string, []any) {
	filter, restricted := authorization.ScopeFilterFromContext(ctx)
	if !restricted {
		return query, args
	}
	position := len(args) + 1
	clause := fmt.Sprintf(`(%s.scope_id = ANY($%d) OR EXISTS (
        WITH RECURSIVE ancestors(id) AS (
            SELECT unnest($%d)
            UNION
            SELECT scope.parent_scope_id FROM scopes scope JOIN ancestors ON ancestors.id = scope.id WHERE scope.parent_scope_id IS NOT NULL
        ) SELECT 1 FROM ancestors WHERE ancestors.id = %s.scope_id
    ))`, alias, position, position, alias)
	return query + " AND " + clause, append(args, filter.ScopeIDs)
}

func exactQuery(query, alias string, ctx context.Context, args ...any) (string, []any) {
	filter, restricted := authorization.ScopeFilterFromContext(ctx)
	if !restricted {
		return query, args
	}
	position := len(args) + 1
	return query + " AND " + alias + ".scope_id = ANY($" + strconv.Itoa(position) + ")", append(args, filter.ScopeIDs)
}

func mapStoreError(err error) error {
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
			return invalid("credential violates a scope constraint")
		}
	}
	return err
}
