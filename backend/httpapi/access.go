package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"opskeeper/backend/audit"
	"opskeeper/backend/authorization"
	"opskeeper/backend/identity"
)

type userManagementService interface {
	CreateUser(context.Context, identity.CreateUserInput) (identity.User, error)
	CreateUserWithOneTimePassword(context.Context, identity.CreateUserInput) (identity.CreateUserResult, error)
	ListUsers(context.Context) ([]identity.User, error)
	GetUser(context.Context, string) (identity.User, error)
	UpdateUser(context.Context, string, identity.UpdateUserInput) (identity.User, error)
	ResetUserPassword(context.Context, string) (string, error)
}

type accessManagementService interface {
	CreateGroup(context.Context, string, authorization.CreateGroupInput, audit.Event) (authorization.Group, error)
	ListGroups(context.Context, string) ([]authorization.Group, error)
	GetGroup(context.Context, string, string) (authorization.Group, error)
	UpdateGroup(context.Context, string, string, authorization.UpdateGroupInput, audit.Event) (authorization.Group, error)
	DeleteGroup(context.Context, string, string, audit.Event) error
	AddGroupMember(context.Context, string, string, string, audit.Event) (authorization.GroupMember, error)
	RemoveGroupMember(context.Context, string, string, string, audit.Event) error
	ListGroupMembers(context.Context, string, string) ([]authorization.GroupMember, error)
	ListVisibleUserIDs(context.Context, string) ([]string, error)
	ListRoles(context.Context, string) ([]authorization.RoleDefinition, error)
	ValidateRoleGrants(context.Context, string, string, []string) error
	CanManageUser(context.Context, string, string) (bool, error)
	CreateRoleBinding(context.Context, string, authorization.GrantRoleInput, audit.Event) (authorization.RoleBinding, error)
	ListRoleBindings(context.Context, string) ([]authorization.RoleBinding, error)
	DeleteRoleBinding(context.Context, string, string, audit.Event) error
	ListResourceRoles(context.Context, string) ([]authorization.ResourceRoleDefinition, error)
	ValidateResourceGrantScope(context.Context, string, string) error
	CreateResourceRoleBinding(context.Context, string, authorization.GrantResourceRoleInput, audit.Event) (authorization.ResourceRoleBinding, error)
	ListResourceRoleBindings(context.Context, string) ([]authorization.ResourceRoleBinding, error)
	DeleteResourceRoleBinding(context.Context, string, string, audit.Event) error
	IsPlatformAdmin(context.Context, string) (bool, error)
}

type auditQueryService interface {
	List(context.Context, []string, int) (audit.Page, error)
}

type accessHandler struct {
	users    userManagementService
	access   accessManagementService
	auditor  audit.Logger
	auditLog auditQueryService
}

type updateUserRequest struct {
	Status      *string `json:"status"`
	DisplayName *string `json:"display_name"`
}

type createUserRequest struct {
	Username     string            `json:"username"`
	Email        string            `json:"email"`
	Phone        string            `json:"phone"`
	DisplayName  string            `json:"display_name"`
	Password     string            `json:"password"`
	PasswordMode string            `json:"password_mode"`
	Grants       []createUserGrant `json:"grants"`
}

type createUserGrant struct {
	ScopeID        string                    `json:"scope_id"`
	RoleID         string                    `json:"role_id"`
	ResourceGrants []createUserResourceGrant `json:"resource_grants"`
}

type createUserResourceGrant struct {
	ResourceID string `json:"resource_id"`
	RoleID     string `json:"role_id"`
}

type createUserResponse struct {
	User            identity.User               `json:"user"`
	Bindings        []authorization.RoleBinding `json:"bindings"`
	OneTimePassword string                      `json:"one_time_password"`
}

type managedUserResponse struct {
	identity.User
	CanManage bool `json:"can_manage"`
}

type createGroupRequest struct {
	ScopeID     string `json:"scope_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type updateGroupRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

type addGroupMemberRequest struct {
	UserID string `json:"user_id"`
}

type createRoleBindingRequest struct {
	SubjectType string `json:"subject_type"`
	SubjectID   string `json:"subject_id"`
	RoleID      string `json:"role_id"`
	ScopeID     string `json:"scope_id"`
}

type createResourceRoleBindingRequest struct {
	SubjectType string `json:"subject_type"`
	SubjectID   string `json:"subject_id"`
	RoleID      string `json:"role_id"`
	ResourceID  string `json:"resource_id"`
}

func registerAccessRoutes(router chi.Router, users userManagementService, access accessManagementService, auditor audit.Logger, auditLog auditQueryService) {
	handler := accessHandler{users: users, access: access, auditor: auditor, auditLog: auditLog}
	router.Route("/users", func(router chi.Router) {
		router.Get("/", handler.listUsers)
		router.Post("/", handler.createUser)
		router.Get("/{userID}", handler.getUser)
		router.Patch("/{userID}", handler.updateUser)
		router.Post("/{userID}/password-reset", handler.resetUserPassword)
	})
	router.Route("/groups", func(router chi.Router) {
		router.Get("/", handler.listGroups)
		router.Post("/", handler.createGroup)
		router.Route("/{groupID}", func(router chi.Router) {
			router.Get("/", handler.getGroup)
			router.Patch("/", handler.updateGroup)
			router.Delete("/", handler.deleteGroup)
			router.Get("/members", handler.listGroupMembers)
			router.Post("/members", handler.addGroupMember)
			router.Delete("/members/{userID}", handler.removeGroupMember)
		})
	})
	router.Get("/roles/", handler.listRoles)
	router.Route("/role-bindings", func(router chi.Router) {
		router.Get("/", handler.listRoleBindings)
		router.Post("/", handler.createRoleBinding)
		router.Delete("/{bindingID}", handler.deleteRoleBinding)
	})
	router.Get("/resource-roles/", handler.listResourceRoles)
	router.Route("/resource-role-bindings", func(router chi.Router) {
		router.Get("/", handler.listResourceRoleBindings)
		router.Post("/", handler.createResourceRoleBinding)
		router.Delete("/{bindingID}", handler.deleteResourceRoleBinding)
	})
}

func registerAuditAuthorizationRoutes(router chi.Router, service authorizationService, auditLog auditQueryService) {
	handler := accessHandler{auditLog: auditLog}
	router.With((authorizationHandler{service: service}).requirePermission(authorization.AuditRead)).Get("/audit-logs", handler.listAuditLogs)
}

func (h accessHandler) listUsers(writer http.ResponseWriter, request *http.Request) {
	users, err := h.users.ListUsers(request.Context())
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	if allowed, checkErr := h.access.IsPlatformAdmin(request.Context(), currentUser(request).ID); checkErr != nil {
		writeAccessError(writer, request, checkErr)
		return
	} else if !allowed {
		visibleIDs, visibleErr := h.access.ListVisibleUserIDs(request.Context(), currentUser(request).ID)
		if visibleErr != nil {
			writeAccessError(writer, request, visibleErr)
			return
		}
		visible := make(map[string]struct{}, len(visibleIDs))
		for _, userID := range visibleIDs {
			visible[userID] = struct{}{}
		}
		filtered := users[:0]
		for _, user := range users {
			if _, ok := visible[user.ID]; ok {
				filtered = append(filtered, user)
			}
		}
		users = filtered
	}
	response := make([]managedUserResponse, 0, len(users))
	for _, user := range users {
		canManage, manageErr := h.access.CanManageUser(request.Context(), currentUser(request).ID, user.ID)
		if manageErr != nil {
			writeAccessError(writer, request, manageErr)
			return
		}
		response = append(response, managedUserResponse{User: user, CanManage: canManage})
	}
	writeJSON(writer, http.StatusOK, response)
}

func (h accessHandler) createUser(writer http.ResponseWriter, request *http.Request) {
	var body createUserRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	actorID := currentUser(request).ID
	if len(body.Grants) == 0 {
		writeError(writer, request, http.StatusBadRequest, "invalid_request", "at least one grant is required")
		return
	}
	for _, grant := range body.Grants {
		if err := h.access.ValidateRoleGrants(request.Context(), actorID, grant.ScopeID, []string{grant.RoleID}); err != nil {
			writeAccessError(writer, request, err)
			return
		}
	}
	if body.PasswordMode != "" && body.PasswordMode != "manual" && body.PasswordMode != "generated" {
		writeError(writer, request, http.StatusBadRequest, "invalid_request", "password_mode must be manual or generated")
		return
	}
	if body.PasswordMode == "generated" {
		body.Password = ""
	}
	userResult, err := h.users.CreateUserWithOneTimePassword(request.Context(), identity.CreateUserInput{Username: body.Username, Email: body.Email, Phone: body.Phone, DisplayName: body.DisplayName, Password: body.Password})
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	user := userResult.User
	bindings := make([]authorization.RoleBinding, 0, len(body.Grants))
	for _, grant := range body.Grants {
		binding, bindingErr := h.access.CreateRoleBinding(request.Context(), actorID, authorization.GrantRoleInput{
			SubjectType: "user",
			SubjectID:   user.ID,
			RoleID:      grant.RoleID,
			ScopeID:     grant.ScopeID,
		}, h.event(request))
		if bindingErr != nil {
			disabled := identity.StatusDisabled
			_, _ = h.users.UpdateUser(request.Context(), user.ID, identity.UpdateUserInput{Status: &disabled})
			writeAccessError(writer, request, bindingErr)
			return
		}
		bindings = append(bindings, binding)
		for _, resourceGrant := range grant.ResourceGrants {
			if binding.RoleName != "ProjectMember" {
				disabled := identity.StatusDisabled
				_, _ = h.users.UpdateUser(request.Context(), user.ID, identity.UpdateUserInput{Status: &disabled})
				writeError(writer, request, http.StatusBadRequest, "invalid_request", "resource grants require the project member role")
				return
			}
			if scopeErr := h.access.ValidateResourceGrantScope(request.Context(), resourceGrant.ResourceID, grant.ScopeID); scopeErr != nil {
				disabled := identity.StatusDisabled
				_, _ = h.users.UpdateUser(request.Context(), user.ID, identity.UpdateUserInput{Status: &disabled})
				writeAccessError(writer, request, scopeErr)
				return
			}
			if _, resourceErr := h.access.CreateResourceRoleBinding(request.Context(), actorID, authorization.GrantResourceRoleInput{
				SubjectType: "user",
				SubjectID:   user.ID,
				RoleID:      resourceGrant.RoleID,
				ResourceID:  resourceGrant.ResourceID,
			}, h.event(request)); resourceErr != nil {
				disabled := identity.StatusDisabled
				_, _ = h.users.UpdateUser(request.Context(), user.ID, identity.UpdateUserInput{Status: &disabled})
				writeAccessError(writer, request, resourceErr)
				return
			}
		}
	}
	event := h.event(request)
	event.TargetType = "user"
	event.TargetID = user.ID
	event.ScopeID = body.Grants[0].ScopeID
	_ = h.record(request, event, "user.create")
	writeJSON(writer, http.StatusCreated, createUserResponse{User: user, Bindings: bindings, OneTimePassword: userResult.OneTimePassword})
}

func (h accessHandler) getUser(writer http.ResponseWriter, request *http.Request) {
	userID := chi.URLParam(request, "userID")
	if !h.requireManagedUser(writer, request, userID) {
		return
	}
	user, err := h.users.GetUser(request.Context(), userID)
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, user)
}

func (h accessHandler) updateUser(writer http.ResponseWriter, request *http.Request) {
	var body updateUserRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	userID := chi.URLParam(request, "userID")
	if !h.requireManagedUser(writer, request, userID) {
		return
	}
	if userID == currentUser(request).ID {
		writeError(writer, request, http.StatusForbidden, "forbidden", "Administrators cannot manage their own account from this page")
		return
	}
	if body.Status == nil && body.DisplayName == nil {
		writeError(writer, request, http.StatusBadRequest, "invalid_request", "At least one user field must be updated")
		return
	}
	if body.Status != nil && *body.Status != identity.StatusDisabled {
		writeError(writer, request, http.StatusBadRequest, "invalid_request", "Only user deletion is supported from user management")
		return
	}
	if userID == currentUser(request).ID && body.Status != nil && *body.Status != identity.StatusActive {
		writeError(writer, request, http.StatusBadRequest, "invalid_request", "You cannot disable or lock your own account")
		return
	}
	user, err := h.users.UpdateUser(request.Context(), userID, identity.UpdateUserInput{
		Status:      body.Status,
		DisplayName: body.DisplayName,
	})
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	event := h.event(request)
	event.TargetType = "user"
	event.TargetID = userID
	event.Details = map[string]any{"status": user.Status, "display_name_updated": body.DisplayName != nil}
	_ = h.record(request, event, "user.update")
	writeJSON(writer, http.StatusOK, user)
}

func (h accessHandler) resetUserPassword(writer http.ResponseWriter, request *http.Request) {
	userID := chi.URLParam(request, "userID")
	if !h.requireManagedUser(writer, request, userID) {
		return
	}
	password, err := h.users.ResetUserPassword(request.Context(), userID)
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	event := h.event(request)
	event.TargetType = "user"
	event.TargetID = userID
	_ = h.record(request, event, "user.password.reset")
	writeJSON(writer, http.StatusOK, map[string]string{"one_time_password": password})
}

func (h accessHandler) listGroups(writer http.ResponseWriter, request *http.Request) {
	groups, err := h.access.ListGroups(request.Context(), currentUser(request).ID)
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, groups)
}

func (h accessHandler) createGroup(writer http.ResponseWriter, request *http.Request) {
	var body createGroupRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	group, err := h.access.CreateGroup(request.Context(), currentUser(request).ID, authorization.CreateGroupInput{ScopeID: body.ScopeID, Name: body.Name, Description: body.Description}, h.event(request))
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, group)
}

func (h accessHandler) getGroup(writer http.ResponseWriter, request *http.Request) {
	group, err := h.access.GetGroup(request.Context(), currentUser(request).ID, chi.URLParam(request, "groupID"))
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, group)
}

func (h accessHandler) updateGroup(writer http.ResponseWriter, request *http.Request) {
	var body updateGroupRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	group, err := h.access.UpdateGroup(request.Context(), currentUser(request).ID, chi.URLParam(request, "groupID"), authorization.UpdateGroupInput{Name: body.Name, Description: body.Description, Status: body.Status}, h.event(request))
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, group)
}

func (h accessHandler) deleteGroup(writer http.ResponseWriter, request *http.Request) {
	if err := h.access.DeleteGroup(request.Context(), currentUser(request).ID, chi.URLParam(request, "groupID"), h.event(request)); err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (h accessHandler) listGroupMembers(writer http.ResponseWriter, request *http.Request) {
	members, err := h.access.ListGroupMembers(request.Context(), currentUser(request).ID, chi.URLParam(request, "groupID"))
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, members)
}

func (h accessHandler) addGroupMember(writer http.ResponseWriter, request *http.Request) {
	var body addGroupMemberRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	member, err := h.access.AddGroupMember(request.Context(), currentUser(request).ID, chi.URLParam(request, "groupID"), body.UserID, h.event(request))
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, member)
}

func (h accessHandler) removeGroupMember(writer http.ResponseWriter, request *http.Request) {
	if err := h.access.RemoveGroupMember(request.Context(), currentUser(request).ID, chi.URLParam(request, "groupID"), chi.URLParam(request, "userID"), h.event(request)); err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (h accessHandler) listRoles(writer http.ResponseWriter, request *http.Request) {
	roles, err := h.access.ListRoles(request.Context(), currentUser(request).ID)
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, roles)
}

func (h accessHandler) listRoleBindings(writer http.ResponseWriter, request *http.Request) {
	bindings, err := h.access.ListRoleBindings(request.Context(), currentUser(request).ID)
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, bindings)
}

func (h accessHandler) createRoleBinding(writer http.ResponseWriter, request *http.Request) {
	var body createRoleBindingRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	binding, err := h.access.CreateRoleBinding(request.Context(), currentUser(request).ID, authorization.GrantRoleInput{SubjectType: body.SubjectType, SubjectID: body.SubjectID, RoleID: body.RoleID, ScopeID: body.ScopeID}, h.event(request))
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, binding)
}

func (h accessHandler) deleteRoleBinding(writer http.ResponseWriter, request *http.Request) {
	if err := h.access.DeleteRoleBinding(request.Context(), currentUser(request).ID, chi.URLParam(request, "bindingID"), h.event(request)); err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (h accessHandler) listResourceRoles(writer http.ResponseWriter, request *http.Request) {
	roles, err := h.access.ListResourceRoles(request.Context(), currentUser(request).ID)
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, roles)
}

func (h accessHandler) listResourceRoleBindings(writer http.ResponseWriter, request *http.Request) {
	bindings, err := h.access.ListResourceRoleBindings(request.Context(), currentUser(request).ID)
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, bindings)
}

func (h accessHandler) createResourceRoleBinding(writer http.ResponseWriter, request *http.Request) {
	var body createResourceRoleBindingRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	binding, err := h.access.CreateResourceRoleBinding(request.Context(), currentUser(request).ID, authorization.GrantResourceRoleInput{
		SubjectType: body.SubjectType,
		SubjectID:   body.SubjectID,
		RoleID:      body.RoleID,
		ResourceID:  body.ResourceID,
	}, h.event(request))
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, binding)
}

func (h accessHandler) deleteResourceRoleBinding(writer http.ResponseWriter, request *http.Request) {
	if err := h.access.DeleteResourceRoleBinding(request.Context(), currentUser(request).ID, chi.URLParam(request, "bindingID"), h.event(request)); err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (h accessHandler) listAuditLogs(writer http.ResponseWriter, request *http.Request) {
	filter, ok := authorization.ScopeFilterFromContext(request.Context())
	if !ok || len(filter.ScopeIDs) == 0 {
		writeError(writer, request, http.StatusForbidden, "forbidden", "You do not have permission for this operation")
		return
	}
	limit, _ := strconv.Atoi(request.URL.Query().Get("limit"))
	page, err := h.auditLog.List(request.Context(), filter.ScopeIDs, limit)
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, page)
}

func (h accessHandler) requirePlatformAdmin(writer http.ResponseWriter, request *http.Request) bool {
	allowed, err := h.access.IsPlatformAdmin(request.Context(), currentUser(request).ID)
	if err != nil {
		writeAccessError(writer, request, err)
		return false
	}
	if !allowed {
		writeError(writer, request, http.StatusForbidden, "forbidden", "Platform administrator permission is required")
		return false
	}
	return true
}

func (h accessHandler) requireManagedUser(writer http.ResponseWriter, request *http.Request, userID string) bool {
	actorID := currentUser(request).ID
	allowed, err := h.access.CanManageUser(request.Context(), actorID, userID)
	if err != nil {
		writeAccessError(writer, request, err)
		return false
	}
	if allowed {
		return true
	}
	writeError(writer, request, http.StatusForbidden, "forbidden", "You do not have permission for this operation")
	return false
}

func (h accessHandler) event(request *http.Request) audit.Event {
	user := currentUser(request)
	return audit.Event{ActorUserID: user.ID, RequestID: middleware.GetReqID(request.Context()), ClientIP: requestClientIP(request)}
}

func (h accessHandler) record(request *http.Request, event audit.Event, action string) error {
	if h.auditor == nil {
		return nil
	}
	event.Action = action
	event.Result = "success"
	return h.auditor.Record(request.Context(), event)
}

func currentUser(request *http.Request) identity.User {
	user, _ := request.Context().Value(authenticatedUserContextKey{}).(identity.User)
	return user
}

func writeAccessError(writer http.ResponseWriter, request *http.Request, err error) {
	switch {
	case errors.Is(err, authorization.ErrForbidden), errors.Is(err, authorization.ErrGrantNotAllowed):
		writeError(writer, request, http.StatusForbidden, "forbidden", "You do not have permission for this operation")
	case errors.Is(err, authorization.ErrNotFound), errors.Is(err, identity.ErrNotFound):
		writeError(writer, request, http.StatusNotFound, "not_found", "Object not found")
	case errors.Is(err, authorization.ErrConflict):
		writeError(writer, request, http.StatusConflict, "conflict", "Object conflicts with existing data")
	case errors.Is(err, identity.ErrConflict):
		writeError(writer, request, http.StatusConflict, "conflict", "User conflicts with existing data")
	case errors.Is(err, authorization.ErrInvalidRole):
		writeError(writer, request, http.StatusBadRequest, "invalid_request", "Role or subject is invalid")
	case errors.Is(err, authorization.ErrInvalidInput):
		writeError(writer, request, http.StatusBadRequest, "invalid_request", "Request contains invalid values")
	case errors.Is(err, identity.ErrNotFound):
		writeError(writer, request, http.StatusNotFound, "not_found", "User not found")
	default:
		var validationError *identity.ValidationError
		if errors.As(err, &validationError) {
			writeError(writer, request, http.StatusBadRequest, "invalid_request", validationError.Message)
			return
		}
		writeError(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}
