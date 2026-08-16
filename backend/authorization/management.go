package authorization

import (
	"context"
	"strings"
	"time"

	"opskeeper/backend/audit"
)

type Group struct {
	ID          string    `json:"id"`
	ScopeID     string    `json:"scope_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type GroupMember struct {
	GroupID   string    `json:"group_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type RoleDefinition struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	ScopeType   string       `json:"scope_type"`
	Builtin     bool         `json:"builtin"`
	Permissions []Permission `json:"permissions"`
}

type RoleBinding struct {
	ID          string    `json:"id"`
	SubjectType string    `json:"subject_type"`
	SubjectID   string    `json:"subject_id"`
	RoleID      string    `json:"role_id"`
	RoleName    string    `json:"role_name"`
	ScopeID     string    `json:"scope_id"`
	ScopeType   string    `json:"scope_type"`
	CreatedAt   time.Time `json:"created_at"`
}

type ResourceRoleDefinition struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Builtin     bool         `json:"builtin"`
	Permissions []Permission `json:"permissions"`
}

type ResourceRoleBinding struct {
	ID           string    `json:"id"`
	SubjectType  string    `json:"subject_type"`
	SubjectID    string    `json:"subject_id"`
	RoleID       string    `json:"role_id"`
	RoleName     string    `json:"role_name"`
	ResourceID   string    `json:"resource_id"`
	ResourceName string    `json:"resource_name"`
	ResourceKind string    `json:"resource_kind"`
	ScopeID      string    `json:"scope_id"`
	CreatedAt    time.Time `json:"created_at"`
}

type CreateGroupInput struct {
	ScopeID     string
	Name        string
	Description string
}

type UpdateGroupInput struct {
	Name        *string
	Description *string
	Status      *string
}

type GrantRoleInput struct {
	SubjectType string
	SubjectID   string
	RoleID      string
	ScopeID     string
}

type GrantResourceRoleInput struct {
	SubjectType string
	SubjectID   string
	RoleID      string
	ResourceID  string
}

type ManagementStore interface {
	CreateGroup(context.Context, CreateGroupInput) (Group, error)
	ListGroups(context.Context, []string) ([]Group, error)
	GetGroup(context.Context, string) (Group, error)
	UpdateGroup(context.Context, string, UpdateGroupInput) (Group, error)
	DeleteGroup(context.Context, string) error
	AddGroupMember(context.Context, string, string, string) (GroupMember, error)
	RemoveGroupMember(context.Context, string, string) error
	ListGroupMembers(context.Context, string) ([]GroupMember, error)
	ListRoles(context.Context) ([]RoleDefinition, error)
	GetRole(context.Context, string) (RoleDefinition, error)
	CreateRoleBinding(context.Context, GrantRoleInput) (RoleBinding, error)
	ListRoleBindings(context.Context, []string) ([]RoleBinding, error)
	GetRoleBinding(context.Context, string) (RoleBinding, error)
	DeleteRoleBinding(context.Context, string) error
	IsPlatformAdmin(context.Context, string) (bool, error)
	ListResourceRoles(context.Context) ([]ResourceRoleDefinition, error)
	GetResourceRole(context.Context, string) (ResourceRoleDefinition, error)
	CreateResourceRoleBinding(context.Context, GrantResourceRoleInput, string) (ResourceRoleBinding, error)
	ListResourceRoleBindings(context.Context, []string) ([]ResourceRoleBinding, error)
	GetResourceRoleBinding(context.Context, string) (ResourceRoleBinding, error)
	DeleteResourceRoleBinding(context.Context, string) error
	ResourceScope(context.Context, string) (string, error)
	SubjectHasScopePermission(context.Context, string, string, Permission, string) (bool, error)
}

type ManagementService struct {
	store         ManagementStore
	authorization *Service
	auditor       audit.Logger
}

func NewManagementService(store ManagementStore, authorizationService *Service, auditor audit.Logger) *ManagementService {
	return &ManagementService{store: store, authorization: authorizationService, auditor: auditor}
}

func (s *ManagementService) CreateGroup(ctx context.Context, actorID string, input CreateGroupInput, event audit.Event) (Group, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if input.ScopeID == "" || input.Name == "" || len([]rune(input.Name)) > 120 || len([]rune(input.Description)) > 500 {
		return Group{}, ErrInvalidInput
	}
	if err := s.authorizeScope(ctx, actorID, MemberGrant, input.ScopeID); err != nil {
		return Group{}, err
	}
	group, err := s.store.CreateGroup(ctx, input)
	if err != nil {
		return Group{}, err
	}
	return group, s.record(ctx, event, "group.create", group.ID, input.ScopeID, map[string]any{"name": group.Name})
}

func (s *ManagementService) ListGroups(ctx context.Context, actorID string) ([]Group, error) {
	filter, err := s.authorization.ScopeFilter(ctx, Subject{UserID: actorID}, MemberGrant)
	if err != nil {
		return nil, err
	}
	if len(filter.ScopeIDs) == 0 {
		return nil, ErrForbidden
	}
	return s.store.ListGroups(ctx, filter.ScopeIDs)
}

func (s *ManagementService) GetGroup(ctx context.Context, actorID, groupID string) (Group, error) {
	group, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		return Group{}, err
	}
	if err := s.authorizeScope(ctx, actorID, MemberGrant, group.ScopeID); err != nil {
		return Group{}, err
	}
	return group, nil
}

func (s *ManagementService) UpdateGroup(ctx context.Context, actorID, groupID string, input UpdateGroupInput, event audit.Event) (Group, error) {
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" || len([]rune(name)) > 120 {
			return Group{}, ErrInvalidInput
		}
		input.Name = &name
	}
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		if len([]rune(description)) > 500 {
			return Group{}, ErrInvalidInput
		}
		input.Description = &description
	}
	if input.Status != nil && *input.Status != "active" && *input.Status != "disabled" {
		return Group{}, ErrInvalidInput
	}
	group, err := s.GetGroup(ctx, actorID, groupID)
	if err != nil {
		return Group{}, err
	}
	updated, err := s.store.UpdateGroup(ctx, group.ID, input)
	if err != nil {
		return Group{}, err
	}
	return updated, s.record(ctx, event, "group.update", group.ID, group.ScopeID, nil)
}

func (s *ManagementService) DeleteGroup(ctx context.Context, actorID, groupID string, event audit.Event) error {
	group, err := s.GetGroup(ctx, actorID, groupID)
	if err != nil {
		return err
	}
	if err := s.store.DeleteGroup(ctx, group.ID); err != nil {
		return err
	}
	return s.record(ctx, event, "group.delete", group.ID, group.ScopeID, nil)
}

func (s *ManagementService) AddGroupMember(ctx context.Context, actorID, groupID, userID string, event audit.Event) (GroupMember, error) {
	group, err := s.GetGroup(ctx, actorID, groupID)
	if err != nil {
		return GroupMember{}, err
	}
	member, err := s.store.AddGroupMember(ctx, group.ID, userID, actorID)
	if err != nil {
		return GroupMember{}, err
	}
	return member, s.record(ctx, event, "group.member.add", group.ID, group.ScopeID, map[string]any{"user_id": userID})
}

func (s *ManagementService) RemoveGroupMember(ctx context.Context, actorID, groupID, userID string, event audit.Event) error {
	group, err := s.GetGroup(ctx, actorID, groupID)
	if err != nil {
		return err
	}
	if err := s.store.RemoveGroupMember(ctx, group.ID, userID); err != nil {
		return err
	}
	return s.record(ctx, event, "group.member.remove", group.ID, group.ScopeID, map[string]any{"user_id": userID})
}

func (s *ManagementService) ListGroupMembers(ctx context.Context, actorID, groupID string) ([]GroupMember, error) {
	group, err := s.GetGroup(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	return s.store.ListGroupMembers(ctx, group.ID)
}

func (s *ManagementService) ListRoles(ctx context.Context, actorID string) ([]RoleDefinition, error) {
	filter, err := s.authorization.ScopeFilter(ctx, Subject{UserID: actorID}, MemberGrant)
	if err != nil {
		return nil, err
	}
	if len(filter.ScopeIDs) == 0 {
		return nil, ErrForbidden
	}
	return s.store.ListRoles(ctx)
}

func (s *ManagementService) CreateRoleBinding(ctx context.Context, actorID string, input GrantRoleInput, event audit.Event) (RoleBinding, error) {
	role, err := s.store.GetRole(ctx, input.RoleID)
	if err != nil {
		return RoleBinding{}, err
	}
	if input.SubjectType != "user" && input.SubjectType != "group" {
		return RoleBinding{}, ErrInvalidRole
	}
	if strings.TrimSpace(input.SubjectID) == "" || strings.TrimSpace(input.RoleID) == "" || strings.TrimSpace(input.ScopeID) == "" {
		return RoleBinding{}, ErrInvalidInput
	}
	if err := s.authorizeScope(ctx, actorID, MemberGrant, input.ScopeID); err != nil {
		return RoleBinding{}, ErrGrantNotAllowed
	}
	for _, permission := range role.Permissions {
		if err := s.authorizeScope(ctx, actorID, permission, input.ScopeID); err != nil {
			return RoleBinding{}, ErrGrantNotAllowed
		}
	}
	binding, err := s.store.CreateRoleBinding(ctx, input)
	if err != nil {
		return RoleBinding{}, err
	}
	return binding, s.record(ctx, event, "role_binding.create", binding.ID, input.ScopeID, map[string]any{
		"role": role.Name, "subject_type": input.SubjectType, "subject_id": input.SubjectID,
	})
}

func (s *ManagementService) ListRoleBindings(ctx context.Context, actorID string) ([]RoleBinding, error) {
	filter, err := s.authorization.ScopeFilter(ctx, Subject{UserID: actorID}, MemberGrant)
	if err != nil {
		return nil, err
	}
	if len(filter.ScopeIDs) == 0 {
		return nil, ErrForbidden
	}
	return s.store.ListRoleBindings(ctx, filter.ScopeIDs)
}

func (s *ManagementService) DeleteRoleBinding(ctx context.Context, actorID, bindingID string, event audit.Event) error {
	binding, err := s.store.GetRoleBinding(ctx, bindingID)
	if err != nil {
		return err
	}
	if err := s.authorizeScope(ctx, actorID, MemberGrant, binding.ScopeID); err != nil {
		return err
	}
	if err := s.store.DeleteRoleBinding(ctx, binding.ID); err != nil {
		return err
	}
	return s.record(ctx, event, "role_binding.delete", binding.ID, binding.ScopeID, nil)
}

func (s *ManagementService) IsPlatformAdmin(ctx context.Context, userID string) (bool, error) {
	return s.store.IsPlatformAdmin(ctx, userID)
}

func (s *ManagementService) authorizeScope(ctx context.Context, actorID string, permission Permission, scopeID string) error {
	if strings.TrimSpace(actorID) == "" || strings.TrimSpace(scopeID) == "" {
		return ErrInvalidSubject
	}
	return s.authorization.Authorize(ctx, Subject{UserID: actorID}, permission, scopeID)
}

func (s *ManagementService) record(ctx context.Context, event audit.Event, action, targetID, scopeID string, details map[string]any) error {
	if s.auditor == nil {
		return nil
	}
	event.Action = action
	event.TargetID = targetID
	event.ScopeID = scopeID
	event.Result = "success"
	event.Details = details
	if strings.HasPrefix(action, "group.") {
		event.TargetType = "group"
	} else if strings.HasPrefix(action, "resource_role_binding.") {
		event.TargetType = "resource_role_binding"
	} else if strings.HasPrefix(action, "role_binding.") {
		event.TargetType = "role_binding"
	}
	return s.auditor.Record(ctx, event)
}
