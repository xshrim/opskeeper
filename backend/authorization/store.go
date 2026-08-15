package authorization

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const bootstrapAdvisoryLockID int64 = 0x4f50534b42535452

type store struct {
	pool *pgxpool.Pool
}

var _ Store = (*store)(nil)

func NewStore(pool *pgxpool.Pool) Store {
	return &store{pool: pool}
}

func (s *store) ResolveScopes(ctx context.Context, subject Subject, permission Permission) (ScopeFilter, error) {
	rows, err := s.pool.Query(ctx, `
		WITH RECURSIVE granted(scope_id) AS (
			SELECT scope.id
			  FROM role_bindings binding
			  JOIN roles role ON role.id = binding.role_id
			  JOIN role_permissions role_permission ON role_permission.role_id = role.id
			  JOIN scopes scope ON scope.id = binding.scope_id
			  JOIN users ON users.id = binding.subject_id
			 WHERE binding.subject_type = 'user'
			   AND binding.subject_id = $1::uuid
			   AND role_permission.permission = $2
			   AND users.status = 'active'
			   AND users.deleted_at IS NULL
			   AND scope.status = 'active'
			   AND scope.deleted_at IS NULL
			 UNION
			SELECT child.id
			  FROM scopes child
			  JOIN granted parent ON parent.scope_id = child.parent_scope_id
			 WHERE child.status = 'active' AND child.deleted_at IS NULL
		)
		SELECT scope_id::text FROM granted ORDER BY scope_id`, subject.UserID, string(permission))
	if err != nil {
		return ScopeFilter{}, fmt.Errorf("resolve authorization scopes: %w", err)
	}
	defer rows.Close()
	filter := ScopeFilter{SubjectID: subject.UserID, Permission: permission, ScopeIDs: make([]string, 0)}
	for rows.Next() {
		var scopeID string
		if err := rows.Scan(&scopeID); err != nil {
			return ScopeFilter{}, fmt.Errorf("scan authorization scope: %w", err)
		}
		filter.ScopeIDs = append(filter.ScopeIDs, scopeID)
	}
	if err := rows.Err(); err != nil {
		return ScopeFilter{}, fmt.Errorf("iterate authorization scopes: %w", err)
	}
	return filter, nil
}

func (s *store) EnsureBootstrapAdmin(ctx context.Context, userID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin bootstrap role binding: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", bootstrapAdvisoryLockID); err != nil {
		return fmt.Errorf("lock bootstrap role binding: %w", err)
	}
	var users int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM users WHERE deleted_at IS NULL").Scan(&users); err != nil {
		return fmt.Errorf("count bootstrap users: %w", err)
	}
	if users != 1 {
		return ErrBootstrapNotAllowed
	}
	var platformScopeID string
	if err := tx.QueryRow(ctx, `
		SELECT scope.id::text
		  FROM platforms platform
		  JOIN scopes scope ON scope.id = platform.scope_id
		 WHERE platform.code = 'default' AND platform.deleted_at IS NULL
		   AND scope.deleted_at IS NULL`).Scan(&platformScopeID); err != nil {
		return mapStoreError(err)
	}
	var roleID string
	if err := tx.QueryRow(ctx, "SELECT id::text FROM roles WHERE name = 'PlatformAdmin' AND builtin").Scan(&roleID); err != nil {
		return mapStoreError(err)
	}
	var onlyUserID string
	if err := tx.QueryRow(ctx, "SELECT id::text FROM users WHERE deleted_at IS NULL").Scan(&onlyUserID); err != nil {
		return fmt.Errorf("find bootstrap user: %w", err)
	}
	if onlyUserID != userID {
		return ErrBootstrapNotAllowed
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO role_bindings (subject_type, subject_id, role_id, scope_id)
		VALUES ('user', $1::uuid, $2::uuid, $3::uuid)
		ON CONFLICT (subject_type, subject_id, role_id, scope_id) DO NOTHING`, userID, roleID, platformScopeID); err != nil {
		return mapStoreError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit bootstrap role binding: %w", err)
	}
	return nil
}

func mapStoreError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrBootstrapNotAllowed
	}
	return err
}
