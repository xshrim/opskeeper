package authorization

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (s *managementStore) ListResourceRoles(ctx context.Context) ([]ResourceRoleDefinition, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT role.id::text, role.name, role.builtin,
		       COALESCE(array_agg(permission.permission ORDER BY permission.permission)
		       FILTER (WHERE permission.permission IS NOT NULL), '{}')
		  FROM resource_roles role
		  LEFT JOIN resource_role_permissions permission ON permission.role_id = role.id
		 GROUP BY role.id ORDER BY role.name`)
	if err != nil {
		return nil, fmt.Errorf("list resource roles: %w", err)
	}
	defer rows.Close()
	items := make([]ResourceRoleDefinition, 0)
	for rows.Next() {
		var item ResourceRoleDefinition
		var permissions []string
		if err := rows.Scan(&item.ID, &item.Name, &item.Builtin, &permissions); err != nil {
			return nil, fmt.Errorf("scan resource role: %w", err)
		}
		for _, permission := range permissions {
			item.Permissions = append(item.Permissions, Permission(permission))
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *managementStore) GetResourceRole(ctx context.Context, id string) (ResourceRoleDefinition, error) {
	items, err := s.ListResourceRoles(ctx)
	if err != nil {
		return ResourceRoleDefinition{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return ResourceRoleDefinition{}, ErrInvalidRole
}

func (s *managementStore) CreateResourceRoleBinding(ctx context.Context, input GrantResourceRoleInput, actorID string) (ResourceRoleBinding, error) {
	var subjectExists bool
	if input.SubjectType == "user" {
		err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1::uuid AND status = 'active' AND deleted_at IS NULL)`, input.SubjectID).Scan(&subjectExists)
		if err != nil {
			return ResourceRoleBinding{}, err
		}
	} else {
		err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM groups WHERE id = $1::uuid AND status = 'active' AND deleted_at IS NULL)`, input.SubjectID).Scan(&subjectExists)
		if err != nil {
			return ResourceRoleBinding{}, err
		}
	}
	if !subjectExists {
		return ResourceRoleBinding{}, ErrNotFound
	}
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO resource_role_bindings (subject_type, subject_id, role_id, resource_id, created_by)
		VALUES ($1, $2::uuid, $3::uuid, $4::uuid, $5::uuid) RETURNING id::text`,
		input.SubjectType, input.SubjectID, input.RoleID, input.ResourceID, actorID).Scan(&id)
	if err != nil {
		return ResourceRoleBinding{}, mapManagementError(err)
	}
	return s.GetResourceRoleBinding(ctx, id)
}

func (s *managementStore) ListResourceRoleBindings(ctx context.Context, scopeIDs []string) ([]ResourceRoleBinding, error) {
	rows, err := s.pool.Query(ctx, resourceRoleBindingSelect+`
		 AND resource.scope_id = ANY($1::uuid[])
		 ORDER BY binding.created_at DESC, binding.id`, scopeIDs)
	if err != nil {
		return nil, fmt.Errorf("list resource role bindings: %w", err)
	}
	defer rows.Close()
	return scanResourceRoleBindings(rows)
}

func (s *managementStore) GetResourceRoleBinding(ctx context.Context, id string) (ResourceRoleBinding, error) {
	rows, err := s.pool.Query(ctx, resourceRoleBindingSelect+" AND binding.id = $1::uuid", id)
	if err != nil {
		return ResourceRoleBinding{}, err
	}
	defer rows.Close()
	items, err := scanResourceRoleBindings(rows)
	if err != nil {
		return ResourceRoleBinding{}, err
	}
	if len(items) == 0 {
		return ResourceRoleBinding{}, ErrNotFound
	}
	return items[0], nil
}

func (s *managementStore) DeleteResourceRoleBinding(ctx context.Context, id string) error {
	command, err := s.pool.Exec(ctx, `DELETE FROM resource_role_bindings WHERE id = $1::uuid`, id)
	if err != nil {
		return mapManagementError(err)
	}
	if command.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *managementStore) ResourceScope(ctx context.Context, resourceID string) (string, error) {
	var scopeID string
	if err := s.pool.QueryRow(ctx, `SELECT scope_id::text FROM resources WHERE id = $1::uuid AND deleted_at IS NULL`, resourceID).Scan(&scopeID); err != nil {
		return "", mapManagementError(err)
	}
	return scopeID, nil
}

func (s *managementStore) SubjectHasScopePermission(ctx context.Context, subjectType, subjectID string, permission Permission, scopeID string) (bool, error) {
	var allowed bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			  FROM role_bindings binding
			  JOIN role_permissions role_permission ON role_permission.role_id = binding.role_id
			 WHERE role_permission.permission = $3
			   AND resource_scope_contains(binding.scope_id, $4::uuid)
			   AND (
				($1 = 'user' AND (
					(binding.subject_type = 'user' AND binding.subject_id = $2::uuid)
					OR (binding.subject_type = 'group' AND EXISTS (
						SELECT 1 FROM group_members member
						JOIN groups group_record ON group_record.id = member.group_id
						WHERE member.group_id = binding.subject_id AND member.user_id = $2::uuid
						  AND group_record.status = 'active' AND group_record.deleted_at IS NULL
					))
				))
				OR ($1 = 'group' AND binding.subject_type = 'group' AND binding.subject_id = $2::uuid)
			   )
		)`, subjectType, subjectID, string(permission), scopeID).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("check resource role subject scope permission: %w", err)
	}
	return allowed, nil
}

const resourceRoleBindingSelect = `
	SELECT binding.id::text, binding.subject_type, binding.subject_id::text,
	       binding.role_id::text, role.name, binding.resource_id::text,
	       resource.name, resource.kind, resource.scope_id::text, binding.created_at
	  FROM resource_role_bindings binding
	  JOIN resource_roles role ON role.id = binding.role_id
	  JOIN resources resource ON resource.id = binding.resource_id
	 WHERE resource.deleted_at IS NULL`

func scanResourceRoleBindings(rows pgx.Rows) ([]ResourceRoleBinding, error) {
	items := make([]ResourceRoleBinding, 0)
	for rows.Next() {
		var item ResourceRoleBinding
		if err := rows.Scan(&item.ID, &item.SubjectType, &item.SubjectID, &item.RoleID,
			&item.RoleName, &item.ResourceID, &item.ResourceName, &item.ResourceKind,
			&item.ScopeID, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan resource role binding: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
