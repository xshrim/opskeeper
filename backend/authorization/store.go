package authorization

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const bootstrapAdvisoryLockID int64 = 0x4f50534b42535452

type store struct {
	pool  *pgxpool.Pool
	cache ScopeCache
}

var _ Store = (*store)(nil)

func NewStore(pool *pgxpool.Pool, caches ...ScopeCache) Store {
	var cache ScopeCache
	if len(caches) > 0 {
		cache = caches[0]
	}
	return &store{pool: pool, cache: cache}
}

func (s *store) ResolveScopes(ctx context.Context, subject Subject, permission Permission) (ScopeFilter, error) {
	var revision int64
	if err := s.pool.QueryRow(ctx, "SELECT revision FROM authorization_revision WHERE id = true").Scan(&revision); err != nil {
		return ScopeFilter{}, fmt.Errorf("read authorization revision: %w", err)
	}
	cacheKey := fmt.Sprintf("opskeeper:authorization:%d:%s:%s", revision, subject.UserID, permission)
	if s.cache != nil {
		if cached, found, err := s.cache.Get(ctx, cacheKey); err == nil && found {
			return cached, nil
		}
	}
	filter, err := s.resolveScopes(ctx, subject, permission)
	if err != nil {
		return ScopeFilter{}, err
	}
	if s.cache != nil {
		_ = s.cache.Set(ctx, cacheKey, filter, 10*time.Minute)
	}
	return filter, nil
}

func (s *store) resolveScopes(ctx context.Context, subject Subject, permission Permission) (ScopeFilter, error) {
	rows, err := s.pool.Query(ctx, `
		WITH RECURSIVE active_ancestors(scope_id, ancestor_id, ancestor_status, ancestor_deleted_at) AS (
			SELECT id, id, status, deleted_at
			  FROM scopes
			 UNION ALL
			SELECT chain.scope_id, parent.id, parent.status, parent.deleted_at
			  FROM active_ancestors chain
			  JOIN scopes current_scope ON current_scope.id = chain.ancestor_id
			  JOIN scopes parent ON parent.id = current_scope.parent_scope_id
		), eligible(scope_id) AS (
			SELECT scope.id
			  FROM scopes scope
			 WHERE scope.status = 'active'
			   AND scope.deleted_at IS NULL
			   AND NOT EXISTS (
					SELECT 1
					  FROM active_ancestors chain
					 WHERE chain.scope_id = scope.id
					   AND (chain.ancestor_status <> 'active' OR chain.ancestor_deleted_at IS NOT NULL)
				)
		), granted(scope_id) AS (
			SELECT scope.id
			  FROM role_bindings binding
			  JOIN roles role ON role.id = binding.role_id
			  JOIN role_permissions role_permission ON role_permission.role_id = role.id
			  JOIN scopes scope ON scope.id = binding.scope_id
			  JOIN eligible ON eligible.scope_id = scope.id
			 WHERE (
				binding.subject_type = 'user'
			   AND EXISTS (
					SELECT 1 FROM users
					 WHERE users.id = binding.subject_id
					   AND users.id = $1::uuid
					   AND users.status = 'active'
					   AND users.deleted_at IS NULL
				)
			    OR binding.subject_type = 'group'
			   AND EXISTS (
					SELECT 1
					  FROM group_members member
					  JOIN groups group_record ON group_record.id = member.group_id
					  JOIN users ON users.id = member.user_id
					 WHERE member.group_id = binding.subject_id
					   AND member.user_id = $1::uuid
					   AND group_record.status = 'active'
					   AND group_record.deleted_at IS NULL
					   AND users.status = 'active'
					   AND users.deleted_at IS NULL
				)
			   )
			   AND role_permission.permission = $2
			   AND scope.status = 'active'
			   AND scope.deleted_at IS NULL
			 UNION
			SELECT child.id
			  FROM scopes child
			  JOIN granted parent ON parent.scope_id = child.parent_scope_id
			  JOIN eligible ON eligible.scope_id = child.id
		), visible(scope_id) AS (
			SELECT scope_id FROM granted
			 UNION
			SELECT scope.parent_scope_id
			  FROM scopes scope
			  JOIN visible child ON child.scope_id = scope.id
			 WHERE $2 = 'organization:read' AND scope.parent_scope_id IS NOT NULL
		)
		SELECT scope_id::text FROM visible ORDER BY scope_id`, subject.UserID, string(permission))
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

func (s *store) ResolveResourceAccess(ctx context.Context, subject Subject, permission Permission) (ResourceFilter, error) {
	scopes, err := s.ResolveScopes(ctx, subject, permission)
	if err != nil {
		return ResourceFilter{}, err
	}
	rows, err := s.pool.Query(ctx, `
		WITH RECURSIVE scope_chain(scope_id, ancestor_id, status, deleted_at) AS (
			SELECT id, id, status, deleted_at FROM scopes
			 UNION ALL
			SELECT chain.scope_id, parent.id, parent.status, parent.deleted_at
			  FROM scope_chain chain
			  JOIN scopes current_scope ON current_scope.id = chain.ancestor_id
			  JOIN scopes parent ON parent.id = current_scope.parent_scope_id
		)
		SELECT DISTINCT binding.resource_id::text
		  FROM resource_role_bindings binding
		  JOIN resource_roles role ON role.id = binding.role_id
		  JOIN resource_role_permissions role_permission ON role_permission.role_id = role.id
		  JOIN resources resource ON resource.id = binding.resource_id AND resource.deleted_at IS NULL
		 WHERE role_permission.permission = $2
		   AND NOT EXISTS (
			SELECT 1 FROM scope_chain chain
			 WHERE chain.scope_id = resource.scope_id
			   AND (chain.status <> 'active' OR chain.deleted_at IS NOT NULL)
		   )
		   AND (
			(binding.subject_type = 'user' AND binding.subject_id = $1::uuid
			 AND EXISTS (SELECT 1 FROM users WHERE id = $1::uuid AND status = 'active' AND deleted_at IS NULL))
			 OR
			(binding.subject_type = 'group' AND EXISTS (
				SELECT 1 FROM group_members member
				JOIN groups group_record ON group_record.id = member.group_id
				JOIN users user_record ON user_record.id = member.user_id
				WHERE member.group_id = binding.subject_id AND member.user_id = $1::uuid
				  AND group_record.status = 'active' AND group_record.deleted_at IS NULL
				  AND user_record.status = 'active' AND user_record.deleted_at IS NULL
			))
		   )
		 ORDER BY binding.resource_id::text`, subject.UserID, string(permission))
	if err != nil {
		return ResourceFilter{}, fmt.Errorf("resolve resource authorization: %w", err)
	}
	defer rows.Close()
	filter := ResourceFilter{SubjectID: subject.UserID, Permission: permission, ScopeIDs: scopes.ScopeIDs, ResourceIDs: make([]string, 0)}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return ResourceFilter{}, fmt.Errorf("scan resource authorization: %w", err)
		}
		filter.ResourceIDs = append(filter.ResourceIDs, id)
	}
	if err := rows.Err(); err != nil {
		return ResourceFilter{}, fmt.Errorf("iterate resource authorization: %w", err)
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
