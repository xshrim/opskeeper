package authorization

import (
	"context"
	"strings"

	"opskeeper/backend/audit"
)

func (s *ManagementService) ListResourceRoles(ctx context.Context, actorID string) ([]ResourceRoleDefinition, error) {
	filter, err := s.authorization.ScopeFilter(ctx, Subject{UserID: actorID}, MemberGrant)
	if err != nil {
		return nil, err
	}
	if len(filter.ScopeIDs) == 0 {
		return nil, ErrForbidden
	}
	return s.store.ListResourceRoles(ctx)
}

func (s *ManagementService) CreateResourceRoleBinding(ctx context.Context, actorID string, input GrantResourceRoleInput, event audit.Event) (ResourceRoleBinding, error) {
	input.SubjectType = strings.TrimSpace(input.SubjectType)
	input.SubjectID = strings.TrimSpace(input.SubjectID)
	input.RoleID = strings.TrimSpace(input.RoleID)
	input.ResourceID = strings.TrimSpace(input.ResourceID)
	if (input.SubjectType != "user" && input.SubjectType != "group") || input.SubjectID == "" || input.RoleID == "" || input.ResourceID == "" {
		return ResourceRoleBinding{}, ErrInvalidInput
	}
	role, err := s.store.GetResourceRole(ctx, input.RoleID)
	if err != nil {
		return ResourceRoleBinding{}, err
	}
	scopeID, err := s.store.ResourceScope(ctx, input.ResourceID)
	if err != nil {
		return ResourceRoleBinding{}, err
	}
	if err := s.authorizeScope(ctx, actorID, MemberGrant, scopeID); err != nil {
		return ResourceRoleBinding{}, ErrGrantNotAllowed
	}
	for _, permission := range role.Permissions {
		if err := s.authorizeScope(ctx, actorID, permission, scopeID); err != nil {
			return ResourceRoleBinding{}, ErrGrantNotAllowed
		}
	}
	eligible, err := s.store.SubjectHasScopePermission(ctx, input.SubjectType, input.SubjectID, OrganizationRead, scopeID)
	if err != nil {
		return ResourceRoleBinding{}, err
	}
	if !eligible {
		return ResourceRoleBinding{}, ErrInvalidInput
	}
	binding, err := s.store.CreateResourceRoleBinding(ctx, input, actorID)
	if err != nil {
		return ResourceRoleBinding{}, err
	}
	return binding, s.record(ctx, event, "resource_role_binding.create", binding.ID, scopeID, map[string]any{
		"role": binding.RoleName, "resource_id": binding.ResourceID,
		"subject_type": binding.SubjectType, "subject_id": binding.SubjectID,
	})
}

func (s *ManagementService) ListResourceRoleBindings(ctx context.Context, actorID string) ([]ResourceRoleBinding, error) {
	filter, err := s.authorization.ScopeFilter(ctx, Subject{UserID: actorID}, MemberGrant)
	if err != nil {
		return nil, err
	}
	if len(filter.ScopeIDs) == 0 {
		return nil, ErrForbidden
	}
	return s.store.ListResourceRoleBindings(ctx, filter.ScopeIDs)
}

func (s *ManagementService) DeleteResourceRoleBinding(ctx context.Context, actorID, bindingID string, event audit.Event) error {
	binding, err := s.store.GetResourceRoleBinding(ctx, bindingID)
	if err != nil {
		return err
	}
	if err := s.authorizeScope(ctx, actorID, MemberGrant, binding.ScopeID); err != nil {
		return err
	}
	if err := s.store.DeleteResourceRoleBinding(ctx, binding.ID); err != nil {
		return err
	}
	return s.record(ctx, event, "resource_role_binding.delete", binding.ID, binding.ScopeID, map[string]any{
		"role": binding.RoleName, "resource_id": binding.ResourceID,
	})
}
