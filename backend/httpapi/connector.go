package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"opskeeper/backend/audit"
	"opskeeper/backend/authorization"
	"opskeeper/backend/connector"
	"opskeeper/backend/resource"
)

type connectorService interface {
	Test(context.Context, string, string) (connector.Check, error)
	Latest(context.Context, string) (connector.Check, error)
}

type connectorHandler struct {
	service connectorService
	auditor audit.Logger
}

func registerConnectorRoutes(router chi.Router, service connectorService, auditor audit.Logger, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	if service == nil {
		return
	}
	handler := connectorHandler{service: service, auditor: auditor}
	guard := func(permission authorization.Permission) func(http.Handler) http.Handler {
		if requirePermission == nil {
			return func(next http.Handler) http.Handler { return next }
		}
		return requirePermission(permission)
	}
	router.With(guard(authorization.ResourceUse)).Post("/resources/{resourceID}/connection-tests", handler.test)
	router.With(guard(authorization.ResourceRead)).Get("/resources/{resourceID}/connection-tests/latest", handler.latest)
}

func (h connectorHandler) test(writer http.ResponseWriter, request *http.Request) {
	check, err := h.service.Test(request.Context(), currentUser(request).ID, chi.URLParam(request, "resourceID"))
	if err != nil {
		writeConnectorError(writer, request, err)
		return
	}
	if h.auditor != nil {
		result := check.Status
		_ = h.auditor.Record(request.Context(), audit.Event{
			ActorUserID: currentUser(request).ID, Action: "resource.connection_test", TargetType: "resource",
			TargetID: check.ResourceID, Result: result, RequestID: middleware.GetReqID(request.Context()),
			ClientIP: requestClientIP(request), Details: map[string]any{
				"error_category": check.ErrorCategory, "latency_ms": check.LatencyMS, "capabilities": check.Capabilities,
			},
		})
	}
	writeJSON(writer, http.StatusOK, check)
}

func (h connectorHandler) latest(writer http.ResponseWriter, request *http.Request) {
	check, err := h.service.Latest(request.Context(), chi.URLParam(request, "resourceID"))
	if err != nil {
		writeConnectorError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, check)
}

func writeConnectorError(writer http.ResponseWriter, request *http.Request, err error) {
	switch {
	case errors.Is(err, authorization.ErrForbidden):
		writeError(writer, request, http.StatusForbidden, "forbidden", "You do not have permission for this operation")
	case errors.Is(err, resource.ErrNotFound):
		writeError(writer, request, http.StatusNotFound, "not_found", "Resource not found")
	case errors.Is(err, connector.ErrNotFound):
		writeError(writer, request, http.StatusNotFound, "connection_check_not_found", "No connection test has been recorded")
	case errors.Is(err, connector.ErrInvalid):
		writeError(writer, request, http.StatusBadRequest, "invalid_request", err.Error())
	default:
		writeError(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}
