package authorization

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type managementStore struct {
	pool *pgxpool.Pool
}

var _ ManagementStore = (*managementStore)(nil)

func NewManagementStore(pool *pgxpool.Pool) ManagementStore {
	return &managementStore{pool: pool}
}

func (s *managementStore) CreateGroup(ctx context.Context, input CreateGroupInput) (Group, error) {
	var group Group
	err := s.pool.QueryRow(ctx, `
		INSERT INTO groups (scope_id, name, description)
		VALUES ($1::uuid, $2, $3)
		RETURNING id::text, scope_id::text, name, description, status, created_at, updated_at`,
		input.ScopeID, input.Name, input.Description).
		Scan(&group.ID, &group.ScopeID, &group.Name, &group.Description, &group.Status, &group.CreatedAt, &group.UpdatedAt)
	return group, mapManagementError(err)
}

func (s *managementStore) ListGroups(ctx context.Context, scopeIDs []string) ([]Group, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, scope_id::text, name, description, status, created_at, updated_at
		  FROM groups
		 WHERE deleted_at IS NULL AND scope_id = ANY($1::uuid[])
		 ORDER BY name, id`, scopeIDs)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close()
	groups := make([]Group, 0)
	for rows.Next() {
		var group Group
		if err := rows.Scan(&group.ID, &group.ScopeID, &group.Name, &group.Description, &group.Status, &group.CreatedAt, &group.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (s *managementStore) GetGroup(ctx context.Context, groupID string) (Group, error) {
	var group Group
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, scope_id::text, name, description, status, created_at, updated_at
		  FROM groups WHERE id = $1::uuid AND deleted_at IS NULL`, groupID).
		Scan(&group.ID, &group.ScopeID, &group.Name, &group.Description, &group.Status, &group.CreatedAt, &group.UpdatedAt)
	return group, mapManagementError(err)
}

func (s *managementStore) UpdateGroup(ctx context.Context, groupID string, input UpdateGroupInput) (Group, error) {
	current, err := s.GetGroup(ctx, groupID)
	if err != nil {
		return Group{}, err
	}
	if input.Name != nil {
		current.Name = *input.Name
	}
	if input.Description != nil {
		current.Description = *input.Description
	}
	if input.Status != nil {
		current.Status = *input.Status
	}
	var group Group
	err = s.pool.QueryRow(ctx, `
		UPDATE groups
		   SET name = $2, description = $3, status = $4, updated_at = now()
		 WHERE id = $1::uuid AND deleted_at IS NULL
		RETURNING id::text, scope_id::text, name, description, status, created_at, updated_at`,
		groupID, current.Name, current.Description, current.Status).
		Scan(&group.ID, &group.ScopeID, &group.Name, &group.Description, &group.Status, &group.CreatedAt, &group.UpdatedAt)
	return group, mapManagementError(err)
}

func (s *managementStore) DeleteGroup(ctx context.Context, groupID string) error {
	command, err := s.pool.Exec(ctx, `UPDATE groups SET deleted_at = now(), updated_at = now() WHERE id = $1::uuid AND deleted_at IS NULL`, groupID)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *managementStore) AddGroupMember(ctx context.Context, groupID, userID, createdBy string) (GroupMember, error) {
	var member GroupMember
	err := s.pool.QueryRow(ctx, `
		INSERT INTO group_members (group_id, user_id, created_by)
		SELECT $1::uuid, users.id, $3::uuid
		  FROM users
		 WHERE users.id = $2::uuid AND users.status = 'active' AND users.deleted_at IS NULL
		RETURNING group_id::text, user_id::text, created_at`, groupID, userID, createdBy).
		Scan(&member.GroupID, &member.UserID, &member.CreatedAt)
	return member, mapManagementError(err)
}

func (s *managementStore) RemoveGroupMember(ctx context.Context, groupID, userID string) error {
	command, err := s.pool.Exec(ctx, `DELETE FROM group_members WHERE group_id = $1::uuid AND user_id = $2::uuid`, groupID, userID)
	if err != nil {
		return fmt.Errorf("remove group member: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *managementStore) ListGroupMembers(ctx context.Context, groupID string) ([]GroupMember, error) {
	rows, err := s.pool.Query(ctx, `SELECT group_id::text, user_id::text, created_at FROM group_members WHERE group_id = $1::uuid ORDER BY created_at, user_id`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list group members: %w", err)
	}
	defer rows.Close()
	members := make([]GroupMember, 0)
	for rows.Next() {
		var member GroupMember
		if err := rows.Scan(&member.GroupID, &member.UserID, &member.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan group member: %w", err)
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (s *managementStore) ListVisibleUserIDs(ctx context.Context, scopeIDs []string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT visible.user_id::text
		  FROM (
			SELECT binding.subject_id AS user_id
			  FROM role_bindings binding
			 WHERE binding.subject_type = 'user' AND binding.scope_id = ANY($1::uuid[])
			UNION
			SELECT member.user_id
			  FROM groups group_record
			  JOIN group_members member ON member.group_id = group_record.id
			 WHERE group_record.scope_id = ANY($1::uuid[])
			   AND group_record.status = 'active' AND group_record.deleted_at IS NULL
		  ) visible
		  JOIN users ON users.id = visible.user_id
		 WHERE users.deleted_at IS NULL
		 ORDER BY visible.user_id::text`, scopeIDs)
	if err != nil {
		return nil, fmt.Errorf("list visible users: %w", err)
	}
	defer rows.Close()
	userIDs := make([]string, 0)
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("scan visible user: %w", err)
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, rows.Err()
}

func (s *managementStore) ListRoles(ctx context.Context) ([]RoleDefinition, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT role.id::text, role.name, role.scope_type, role.builtin,
		       COALESCE(array_agg(role_permission.permission ORDER BY role_permission.permission) FILTER (WHERE role_permission.permission IS NOT NULL), '{}')
		  FROM roles role
		  LEFT JOIN role_permissions role_permission ON role_permission.role_id = role.id
		 GROUP BY role.id ORDER BY role.scope_type, role.name`)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()
	roles := make([]RoleDefinition, 0)
	for rows.Next() {
		var role RoleDefinition
		var permissions []string
		if err := rows.Scan(&role.ID, &role.Name, &role.ScopeType, &role.Builtin, &permissions); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		for _, permission := range permissions {
			role.Permissions = append(role.Permissions, Permission(permission))
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (s *managementStore) ScopeType(ctx context.Context, scopeID string) (string, error) {
	var scopeType string
	if err := s.pool.QueryRow(ctx, `
		SELECT scope_type FROM scopes
		 WHERE id = $1::uuid AND status = 'active' AND deleted_at IS NULL`, scopeID).Scan(&scopeType); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get scope type: %w", err)
	}
	return scopeType, nil
}

func (s *managementStore) ListActiveScopeIDs(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		WITH RECURSIVE scope_chain(scope_id, ancestor_id, ancestor_status, ancestor_deleted_at) AS (
			SELECT id, id, status, deleted_at FROM scopes
			UNION ALL
			SELECT chain.scope_id, parent.id, parent.status, parent.deleted_at
			  FROM scope_chain chain
			  JOIN scopes current_scope ON current_scope.id = chain.ancestor_id
			  JOIN scopes parent ON parent.id = current_scope.parent_scope_id
		)
		SELECT scope.id::text
		  FROM scopes scope
		 WHERE scope.status = 'active' AND scope.deleted_at IS NULL
		   AND NOT EXISTS (
			SELECT 1 FROM scope_chain chain
			 WHERE chain.scope_id = scope.id
			   AND (chain.ancestor_status <> 'active' OR chain.ancestor_deleted_at IS NOT NULL)
		 )
		 ORDER BY scope.id`)
	if err != nil {
		return nil, fmt.Errorf("list active scopes: %w", err)
	}
	defer rows.Close()
	scopeIDs := make([]string, 0)
	for rows.Next() {
		var scopeID string
		if err := rows.Scan(&scopeID); err != nil {
			return nil, fmt.Errorf("scan active scope: %w", err)
		}
		scopeIDs = append(scopeIDs, scopeID)
	}
	return scopeIDs, rows.Err()
}

func (s *managementStore) ListUserRoleBindings(ctx context.Context, userID string) ([]RoleBinding, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT binding.id::text, binding.subject_type, binding.subject_id::text,
		       role.id::text, role.name, binding.scope_id::text, scope.scope_type,
		       binding.created_at
		  FROM role_bindings binding
		  JOIN roles role ON role.id = binding.role_id
		  JOIN scopes scope ON scope.id = binding.scope_id
		 WHERE scope.deleted_at IS NULL
		   AND (
			(binding.subject_type = 'user' AND binding.subject_id = $1::uuid)
			OR (binding.subject_type = 'group' AND EXISTS (
				SELECT 1 FROM group_members member
				JOIN groups group_record ON group_record.id = member.group_id
				WHERE member.group_id = binding.subject_id
				  AND member.user_id = $1::uuid
				  AND group_record.status = 'active'
				  AND group_record.deleted_at IS NULL
			))
		   )
		 ORDER BY binding.created_at, binding.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user role bindings: %w", err)
	}
	defer rows.Close()
	bindings := make([]RoleBinding, 0)
	for rows.Next() {
		var binding RoleBinding
		if err := rows.Scan(&binding.ID, &binding.SubjectType, &binding.SubjectID, &binding.RoleID, &binding.RoleName, &binding.ScopeID, &binding.ScopeType, &binding.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user role binding: %w", err)
		}
		bindings = append(bindings, binding)
	}
	return bindings, rows.Err()
}

func (s *managementStore) GetRole(ctx context.Context, roleID string) (RoleDefinition, error) {
	roles, err := s.ListRoles(ctx)
	if err != nil {
		return RoleDefinition{}, err
	}
	for _, role := range roles {
		if role.ID == roleID {
			return role, nil
		}
	}
	return RoleDefinition{}, ErrInvalidRole
}

func (s *managementStore) CreateRoleBinding(ctx context.Context, input GrantRoleInput) (RoleBinding, error) {
	var binding RoleBinding
	err := s.pool.QueryRow(ctx, `
		INSERT INTO role_bindings (subject_type, subject_id, role_id, scope_id)
		VALUES ($1, $2::uuid, $3::uuid, $4::uuid)
		RETURNING id::text, subject_type, subject_id::text, role_id::text, scope_id::text, created_at`,
		input.SubjectType, input.SubjectID, input.RoleID, input.ScopeID).
		Scan(&binding.ID, &binding.SubjectType, &binding.SubjectID, &binding.RoleID, &binding.ScopeID, &binding.CreatedAt)
	if err != nil {
		return RoleBinding{}, mapManagementError(err)
	}
	full, err := s.GetRoleBinding(ctx, binding.ID)
	return full, err
}

func (s *managementStore) ListRoleBindings(ctx context.Context, scopeIDs []string) ([]RoleBinding, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT binding.id::text, binding.subject_type, binding.subject_id::text, binding.role_id::text,
		       role.name, binding.scope_id::text, scope.scope_type, binding.created_at
		  FROM role_bindings binding
		  JOIN roles role ON role.id = binding.role_id
		  JOIN scopes scope ON scope.id = binding.scope_id
		 WHERE binding.scope_id = ANY($1::uuid[])
		 ORDER BY binding.created_at DESC, binding.id`, scopeIDs)
	if err != nil {
		return nil, fmt.Errorf("list role bindings: %w", err)
	}
	defer rows.Close()
	return scanRoleBindings(rows)
}

func (s *managementStore) GetRoleBinding(ctx context.Context, bindingID string) (RoleBinding, error) {
	var binding RoleBinding
	err := s.pool.QueryRow(ctx, `
		SELECT binding.id::text, binding.subject_type, binding.subject_id::text, binding.role_id::text,
		       role.name, binding.scope_id::text, scope.scope_type, binding.created_at
		  FROM role_bindings binding
		  JOIN roles role ON role.id = binding.role_id
		  JOIN scopes scope ON scope.id = binding.scope_id
		 WHERE binding.id = $1::uuid`, bindingID).
		Scan(&binding.ID, &binding.SubjectType, &binding.SubjectID, &binding.RoleID, &binding.RoleName, &binding.ScopeID, &binding.ScopeType, &binding.CreatedAt)
	return binding, mapManagementError(err)
}

func (s *managementStore) DeleteRoleBinding(ctx context.Context, bindingID string) error {
	command, err := s.pool.Exec(ctx, `DELETE FROM role_bindings WHERE id = $1::uuid`, bindingID)
	if err != nil {
		return fmt.Errorf("delete role binding: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *managementStore) IsPlatformAdmin(ctx context.Context, userID string) (bool, error) {
	return s.hasPlatformRole(ctx, userID, []string{"PlatformAdmin"})
}

func (s *managementStore) HasPlatformRole(ctx context.Context, userID string) (bool, error) {
	return s.hasPlatformRole(ctx, userID, []string{"PlatformAdmin", "PlatformOperator", "PlatformViewer"})
}

func (s *managementStore) hasPlatformRole(ctx context.Context, userID string, roleNames []string) (bool, error) {
	var allowed bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			  FROM role_bindings binding
			  JOIN roles role ON role.id = binding.role_id
			  JOIN scopes scope ON scope.id = binding.scope_id
			 WHERE binding.subject_type = 'user'
			   AND binding.subject_id = $1::uuid
			   AND role.name = ANY($2::text[])
			   AND scope.scope_type = 'platform'
			   AND scope.status = 'active' AND scope.deleted_at IS NULL
			   AND EXISTS (SELECT 1 FROM users WHERE users.id = $1::uuid AND users.status = 'active' AND users.deleted_at IS NULL)
			 UNION ALL
			SELECT 1
			  FROM role_bindings binding
			  JOIN roles role ON role.id = binding.role_id
			  JOIN scopes scope ON scope.id = binding.scope_id
			  JOIN group_members member ON member.group_id = binding.subject_id
			  JOIN groups group_record ON group_record.id = member.group_id
			 WHERE binding.subject_type = 'group'
			   AND member.user_id = $1::uuid
			   AND role.name = ANY($2::text[])
			   AND scope.scope_type = 'platform'
			   AND scope.status = 'active' AND scope.deleted_at IS NULL
			   AND group_record.status = 'active' AND group_record.deleted_at IS NULL
			   AND EXISTS (SELECT 1 FROM users WHERE users.id = $1::uuid AND users.status = 'active' AND users.deleted_at IS NULL)
		)`, userID, roleNames).Scan(&allowed)
	return allowed, err
}

func scanRoleBindings(rows pgx.Rows) ([]RoleBinding, error) {
	bindings := make([]RoleBinding, 0)
	for rows.Next() {
		var binding RoleBinding
		if err := rows.Scan(&binding.ID, &binding.SubjectType, &binding.SubjectID, &binding.RoleID, &binding.RoleName, &binding.ScopeID, &binding.ScopeType, &binding.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan role binding: %w", err)
		}
		bindings = append(bindings, binding)
	}
	return bindings, rows.Err()
}

func mapManagementError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) {
		switch databaseError.Code {
		case "23505":
			return ErrConflict
		case "23503", "23514":
			return ErrNotFound
		}
	}
	return err
}
