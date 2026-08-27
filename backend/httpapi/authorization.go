package httpapi

import (
	"context"
	"net/http"

	"opskeeper/backend/authorization"
	"opskeeper/backend/identity"
)

type authorizationService interface {
	ScopeFilter(context.Context, authorization.Subject, authorization.Permission) (authorization.ScopeFilter, error)
	ResourceFilter(context.Context, authorization.Subject, authorization.Permission) (authorization.ResourceFilter, error)
}

type authorizationHandler struct {
	service authorizationService
}

func (h authorizationHandler) requirePermission(permission authorization.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			user, ok := request.Context().Value(authenticatedUserContextKey{}).(identity.User)
			if !ok {
				writeError(writer, request, http.StatusUnauthorized, "invalid_session", "Authentication is required")
				return
			}
			if resourceScopedPermission(permission) {
				filter, err := h.service.ResourceFilter(request.Context(), authorization.Subject{UserID: user.ID}, permission)
				if err != nil {
					writeError(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error")
					return
				}
				if len(filter.ScopeIDs) == 0 && len(filter.ResourceIDs) == 0 {
					writeError(writer, request, http.StatusForbidden, "forbidden", "You do not have permission for this operation")
					return
				}
				ctx := authorization.WithResourceFilter(request.Context(), filter)
				next.ServeHTTP(writer, request.WithContext(ctx))
				return
			}
			filter, err := h.service.ScopeFilter(request.Context(), authorization.Subject{UserID: user.ID}, permission)
			if err != nil {
				writeError(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error")
				return
			}
			if len(filter.ScopeIDs) == 0 {
				writeError(writer, request, http.StatusForbidden, "forbidden", "You do not have permission for this operation")
				return
			}
			ctx := authorization.WithScopeFilter(request.Context(), filter)
			next.ServeHTTP(writer, request.WithContext(ctx))
		})
	}
}

func resourceScopedPermission(permission authorization.Permission) bool {
	switch permission {
	case authorization.ResourceRead, authorization.ResourceUpdate, authorization.ResourceDelete,
		authorization.ResourceUse, authorization.RelationManage, authorization.DiscoveryRun,
		authorization.AIModelManage,
		authorization.DiagnosisStart, authorization.DiagnosisRead,
		authorization.InspectionManage, authorization.InspectionExecute:
		return true
	default:
		return false
	}
}
