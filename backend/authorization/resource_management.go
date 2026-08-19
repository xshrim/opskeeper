package authorization

import (
	"context"
	"strings"

	"opskeeper/backend/audit"
)

func (s *ManagementService) ListResourceRoles(ctx context.Context, actorID string) ([]ResourceRoleDefinition, error) {
	filter, err := s.memberVisibilityFilter(ctx, actorID)
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
	if err := RequireResourceGrantViewer(s.store, ctx, input.SubjectType, input.SubjectID, scopeID); err != nil {
		return ResourceRoleBinding{}, err
	}
	for _, permission := range role.Permissions {
		if permission == ResourceCreate || permission == ResourceUpdate || permission == ResourceDelete {
			return ResourceRoleBinding{}, ErrGrantNotAllowed
		}
	}
	if err := s.authorizeScope(ctx, actorID, MemberGrant, scopeID); err != nil {
		return ResourceRoleBinding{}, ErrGrantNotAllowed
	}
	for _, permission := range role.Permissions {
		if err := s.authorizeScope(ctx, actorID, permission, scopeID); err != nil {
			return ResourceRoleBinding{}, ErrGrantNotAllowed
		}
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

// ResourceGrantViewerRole returns the only scope role that may receive
// per-resource supplemental permissions for the given resource scope.
func ResourceGrantViewerRole(scopeType string) (string, bool) {
	switch scopeType {
	case "platform":
		return "PlatformViewer", true
	case "team":
		return "TeamViewer", true
	case "project":
		return "ProjectViewer", true
	default:
		return "", false
	}
}

// RequireResourceGrantViewer ensures supplemental resource roles remain bound
// to the corresponding scope-level viewer role rather than becoming a second,
// bypassable access hierarchy.
func RequireResourceGrantViewer(store ManagementStore, ctx context.Context, subjectType, subjectID, scopeID string) error {
	scopeType, err := store.ScopeType(ctx, scopeID)
	if err != nil {
		return err
	}
	if _, ok := ResourceGrantViewerRole(scopeType); !ok {
		return ErrInvalidInput
	}
	eligible, err := store.SubjectHasScopeViewerRole(ctx, subjectType, subjectID, scopeID)
	if err != nil {
		return err
	}
	if !eligible {
		return ErrInvalidInput
	}
	return nil
}

// ValidateResourceGrantScope confirms that a resource selected for a scope
// viewer belongs to the selected scope.
func (s *ManagementService) ValidateResourceGrantScope(ctx context.Context, resourceID, scopeID string) error {
	resourceScopeID, err := s.store.ResourceScope(ctx, strings.TrimSpace(resourceID))
	if err != nil {
		return err
	}
	if resourceScopeID != strings.TrimSpace(scopeID) {
		return ErrInvalidInput
	}
	return nil
}

func (s *ManagementService) ListResourceRoleBindings(ctx context.Context, actorID string) ([]ResourceRoleBinding, error) {
	filter, err := s.memberVisibilityFilter(ctx, actorID)
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
