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
	"opskeeper/backend/connector"
	"opskeeper/backend/resource"
)

type connectorService interface {
	Test(context.Context, string, string) (connector.Check, error)
	Latest(context.Context, string) (connector.Check, error)
}
type dockerDraftConnectorService interface {
	TestDockerDraft(context.Context, connector.DockerDraftInput) (connector.DockerDraftCheck, error)
}
type kubernetesDraftConnectorService interface {
	TestKubernetesDraft(context.Context, connector.KubernetesDraftInput) (connector.KubernetesDraftCheck, error)
}
type hostDraftConnectorService interface {
	TestHostDraft(context.Context, connector.HostDraftInput) (connector.HostDraftCheck, error)
}
type postgresqlDraftConnectorService interface { TestPostgreSQLDraft(context.Context, connector.PostgreSQLDraftInput) (connector.PostgreSQLDraftCheck, error) }
type applicationDiscoveryService interface {
	DiscoverApplicationHostProcesses(context.Context, string, string, int) (connector.ApplicationHostProcesses, error)
	ValidateApplicationHostProcess(context.Context, string, string, int) (connector.ApplicationHostProcessValidation, error)
	DiscoverApplicationDockerContainers(context.Context, string, string, int) (connector.ApplicationDockerContainers, error)
	DiscoverApplicationKubernetesNamespaces(context.Context, string, bool, int) (connector.ApplicationKubernetesNamespaces, error)
	DiscoverApplicationKubernetesWorkloads(context.Context, string, string, int) (connector.ApplicationKubernetesWorkloads, error)
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
	if draft, ok := service.(dockerDraftConnectorService); ok {
		router.With(guard(authorization.ResourceUpdate)).Post("/docker/connection-tests", func(w http.ResponseWriter, r *http.Request) {
			var body connector.DockerDraftInput
			if !decodeRequest(w, r, &body) {
				return
			}
			check, err := draft.TestDockerDraft(r.Context(), body)
			if err != nil {
				writeConnectorError(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, check)
		})
	}
	if draft, ok := service.(kubernetesDraftConnectorService); ok {
		router.With(guard(authorization.ResourceUpdate)).Post("/kubernetes/connection-tests", func(w http.ResponseWriter, r *http.Request) {
			var body connector.KubernetesDraftInput
			if !decodeRequest(w, r, &body) {
				return
			}
			check, err := draft.TestKubernetesDraft(r.Context(), body)
			if err != nil {
				writeConnectorError(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, check)
		})
	}
	if draft, ok := service.(hostDraftConnectorService); ok {
		router.With(guard(authorization.ResourceUpdate)).Post("/host/connection-tests", func(w http.ResponseWriter, r *http.Request) {
			var body connector.HostDraftInput
			if !decodeRequest(w, r, &body) {
				return
			}
			check, err := draft.TestHostDraft(r.Context(), body)
			if err != nil {
				writeConnectorError(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, check)
		})
	}
	if draft, ok := service.(postgresqlDraftConnectorService); ok {
		router.With(guard(authorization.ResourceUpdate)).Post("/postgresql/connection-tests", func(w http.ResponseWriter, r *http.Request) {
			var body connector.PostgreSQLDraftInput; if !decodeRequest(w,r,&body){return}; check,err:=draft.TestPostgreSQLDraft(r.Context(),body); if err!=nil{writeConnectorError(w,r,err);return}; writeJSON(w,http.StatusOK,check)
		})
	}
	if discovery, ok := service.(applicationDiscoveryService); ok {
		router.With(guard(authorization.ResourceUse)).Get("/resources/{resourceID}/application-targets/host-processes", func(w http.ResponseWriter, r *http.Request) {
			limit := queryLimit(r)
			result, err := discovery.DiscoverApplicationHostProcesses(r.Context(), chi.URLParam(r, "resourceID"), r.URL.Query().Get("keyword"), limit)
			if err != nil {
				writeConnectorError(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		})
		router.With(guard(authorization.ResourceUse)).Post("/resources/{resourceID}/application-targets/host-processes/validate", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Keyword string `json:"keyword"`
				PID     int    `json:"pid"`
			}
			if !decodeRequest(w, r, &body) {
				return
			}
			result, err := discovery.ValidateApplicationHostProcess(r.Context(), chi.URLParam(r, "resourceID"), body.Keyword, body.PID)
			if err != nil {
				writeConnectorError(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		})
		router.With(guard(authorization.ResourceUse)).Get("/resources/{resourceID}/application-targets/docker-containers", func(w http.ResponseWriter, r *http.Request) {
			result, err := discovery.DiscoverApplicationDockerContainers(r.Context(), chi.URLParam(r, "resourceID"), r.URL.Query().Get("keyword"), queryLimit(r))
			if err != nil {
				writeConnectorError(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		})
		router.With(guard(authorization.ResourceUse)).Get("/resources/{resourceID}/application-targets/kubernetes-namespaces", func(w http.ResponseWriter, r *http.Request) {
			result, err := discovery.DiscoverApplicationKubernetesNamespaces(r.Context(), chi.URLParam(r, "resourceID"), queryBool(r, "include_system"), queryLimit(r))
			if err != nil {
				writeConnectorError(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		})
		router.With(guard(authorization.ResourceUse)).Get("/resources/{resourceID}/application-targets/kubernetes-workloads", func(w http.ResponseWriter, r *http.Request) {
			result, err := discovery.DiscoverApplicationKubernetesWorkloads(r.Context(), chi.URLParam(r, "resourceID"), r.URL.Query().Get("namespace"), queryLimit(r))
			if err != nil {
				writeConnectorError(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		})
	}
	router.With(guard(authorization.ResourceRead)).Get("/resources/{resourceID}/connection-tests/latest", handler.latest)
}

func queryLimit(request *http.Request) int {
	value, err := strconv.Atoi(request.URL.Query().Get("limit"))
	if err != nil || value <= 0 {
		return 50
	}
	if value > 100 {
		return 100
	}
	return value
}

func queryBool(request *http.Request, name string) bool {
	value, _ := strconv.ParseBool(request.URL.Query().Get(name))
	return value
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
	case errors.Is(err, connector.ErrUnsupported):
		writeError(writer, request, http.StatusBadRequest, "unsupported", err.Error())
	default:
		writeError(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}
