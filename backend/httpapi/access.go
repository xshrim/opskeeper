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
	ListUsers(context.Context) ([]identity.User, error)
	GetUser(context.Context, string) (identity.User, error)
	UpdateUser(context.Context, string, identity.UpdateUserInput) (identity.User, error)
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
	ListRoles(context.Context, string) ([]authorization.RoleDefinition, error)
	CreateRoleBinding(context.Context, string, authorization.GrantRoleInput, audit.Event) (authorization.RoleBinding, error)
	ListRoleBindings(context.Context, string) ([]authorization.RoleBinding, error)
	DeleteRoleBinding(context.Context, string, string, audit.Event) error
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
	DisplayName *string `json:"display_name"`
	Status      *string `json:"status"`
}

type createUserRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
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

func registerAccessRoutes(router chi.Router, users userManagementService, access accessManagementService, auditor audit.Logger, auditLog auditQueryService) {
	handler := accessHandler{users: users, access: access, auditor: auditor, auditLog: auditLog}
	router.Route("/users", func(router chi.Router) {
		router.Get("/", handler.listUsers)
		router.Post("/", handler.createUser)
		router.Get("/{userID}", handler.getUser)
		router.Patch("/{userID}", handler.updateUser)
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
}

func registerAuditAuthorizationRoutes(router chi.Router, service authorizationService, auditLog auditQueryService) {
	handler := accessHandler{auditLog: auditLog}
	router.With((authorizationHandler{service: service}).requirePermission(authorization.AuditRead)).Get("/audit-logs", handler.listAuditLogs)
}

func (h accessHandler) listUsers(writer http.ResponseWriter, request *http.Request) {
	if !h.requirePlatformAdmin(writer, request) {
		return
	}
	users, err := h.users.ListUsers(request.Context())
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, users)
}

func (h accessHandler) createUser(writer http.ResponseWriter, request *http.Request) {
	if !h.requirePlatformAdmin(writer, request) {
		return
	}
	var body createUserRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	user, err := h.users.CreateUser(request.Context(), identity.CreateUserInput{Email: body.Email, DisplayName: body.DisplayName, Password: body.Password})
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	event := h.event(request)
	event.TargetType = "user"
	event.TargetID = user.ID
	_ = h.record(request, event, "user.create")
	writeJSON(writer, http.StatusCreated, user)
}

func (h accessHandler) getUser(writer http.ResponseWriter, request *http.Request) {
	if !h.requirePlatformAdmin(writer, request) {
		return
	}
	user, err := h.users.GetUser(request.Context(), chi.URLParam(request, "userID"))
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, user)
}

func (h accessHandler) updateUser(writer http.ResponseWriter, request *http.Request) {
	if !h.requirePlatformAdmin(writer, request) {
		return
	}
	var body updateUserRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	userID := chi.URLParam(request, "userID")
	user, err := h.users.UpdateUser(request.Context(), userID, identity.UpdateUserInput{DisplayName: body.DisplayName, Status: body.Status})
	if err != nil {
		writeAccessError(writer, request, err)
		return
	}
	event := h.event(request)
	event.TargetType = "user"
	event.TargetID = userID
	event.Details = map[string]any{"status": user.Status}
	_ = h.record(request, event, "user.update")
	writeJSON(writer, http.StatusOK, user)
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
