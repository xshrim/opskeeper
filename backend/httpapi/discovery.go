package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"opskeeper/backend/authorization"
	"opskeeper/backend/discovery"
)

type discoveryService interface {
	Start(context.Context, string, string) (discovery.Run, error)
	Get(context.Context, string) (discovery.Run, error)
	List(context.Context, string) ([]discovery.Run, error)
	Items(context.Context, string) ([]discovery.Item, error)
	Import(context.Context, string, string, discovery.ImportInput) (discovery.ImportResult, error)
}

type discoveryHandler struct{ service discoveryService }

func registerDiscoveryRoutes(router chi.Router, service discoveryService, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	if service == nil {
		return
	}
	handler := discoveryHandler{service: service}
	guard := func(permission authorization.Permission) func(http.Handler) http.Handler {
		if requirePermission == nil {
			return func(next http.Handler) http.Handler { return next }
		}
		return requirePermission(permission)
	}
	router.With(guard(authorization.DiscoveryRun)).Post("/resources/{resourceID}/discoveries", handler.start)
	router.With(guard(authorization.DiscoveryRun)).Get("/resources/{resourceID}/discoveries", handler.list)
	router.Route("/discoveries/{discoveryID}", func(router chi.Router) {
		router.With(guard(authorization.DiscoveryRun)).Get("/", handler.get)
		router.With(guard(authorization.DiscoveryRun)).Get("/items", handler.items)
		router.With(guard(authorization.DiscoveryImport)).Post("/imports", handler.importItems)
	})
}

func (h discoveryHandler) start(writer http.ResponseWriter, request *http.Request) {
	run, err := h.service.Start(request.Context(), currentUser(request).ID, chi.URLParam(request, "resourceID"))
	if err != nil {
		writeDiscoveryError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusAccepted, run)
}

func (h discoveryHandler) list(writer http.ResponseWriter, request *http.Request) {
	runs, err := h.service.List(request.Context(), chi.URLParam(request, "resourceID"))
	if err != nil {
		writeDiscoveryError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, runs)
}

func (h discoveryHandler) get(writer http.ResponseWriter, request *http.Request) {
	run, err := h.service.Get(request.Context(), chi.URLParam(request, "discoveryID"))
	if err != nil {
		writeDiscoveryError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, run)
}

func (h discoveryHandler) items(writer http.ResponseWriter, request *http.Request) {
	items, err := h.service.Items(request.Context(), chi.URLParam(request, "discoveryID"))
	if err != nil {
		writeDiscoveryError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, items)
}

func (h discoveryHandler) importItems(writer http.ResponseWriter, request *http.Request) {
	var body discovery.ImportInput
	if !decodeRequest(writer, request, &body) {
		return
	}
	result, err := h.service.Import(request.Context(), currentUser(request).ID, chi.URLParam(request, "discoveryID"), body)
	if err != nil {
		writeDiscoveryError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func writeDiscoveryError(writer http.ResponseWriter, request *http.Request, err error) {
	switch {
	case errors.Is(err, authorization.ErrForbidden):
		writeError(writer, request, http.StatusForbidden, "forbidden", "You do not have permission for this operation")
	case errors.Is(err, discovery.ErrNotFound):
		writeError(writer, request, http.StatusNotFound, "not_found", "Discovery run not found")
	case errors.Is(err, discovery.ErrConflict):
		writeError(writer, request, http.StatusConflict, "conflict", "Discovery is not ready for this operation")
	case errors.Is(err, discovery.ErrInvalid):
		writeError(writer, request, http.StatusBadRequest, "invalid_request", err.Error())
	default:
		writeError(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}
