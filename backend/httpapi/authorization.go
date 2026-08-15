package httpapi

import (
	"context"
	"net/http"

	"opskeeper/backend/authorization"
	"opskeeper/backend/identity"
)

type authorizationService interface {
	ScopeFilter(context.Context, authorization.Subject, authorization.Permission) (authorization.ScopeFilter, error)
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
