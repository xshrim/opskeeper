package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"opskeeper/backend/authorization"
	"opskeeper/backend/llm"
)

type providerBindingService interface {
	ListBindings(context.Context, string) ([]llm.ScopeProviderBinding, error)
	SetBinding(context.Context, string, string, llm.Purpose, string) (llm.ScopeProviderBinding, error)
	RemoveBinding(context.Context, string, llm.Purpose) error
}
type providerAvailabilityService interface {
	Available(context.Context, string, llm.Purpose) ([]llm.AvailableProvider, error)
}

type providerBindingRequest struct {
	ProviderID string `json:"provider_id"`
}

func registerProviderBindingRoutes(router chi.Router, service providerBindingService, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	if service == nil {
		return
	}
	guard := func(permission authorization.Permission) func(http.Handler) http.Handler {
		if requirePermission == nil {
			return func(next http.Handler) http.Handler { return next }
		}
		return requirePermission(permission)
	}
	h := providerBindingHandler{service: service}
	router.With(guard(authorization.ProviderRead)).Get("/scopes/{scopeID}/provider-bindings", h.list)
	router.With(guard(authorization.ProviderManage)).Put("/scopes/{scopeID}/provider-bindings/{tag}", h.set)
	router.With(guard(authorization.ProviderManage)).Delete("/scopes/{scopeID}/provider-bindings/{tag}", h.remove)
}

func registerProviderAvailabilityRoute(router chi.Router, service providerAvailabilityService, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	if service == nil {
		return
	}
	guard := func(next http.Handler) http.Handler {
		if requirePermission == nil {
			return next
		}
		return requirePermission(authorization.ProviderUse)(next)
	}
	router.With(guard).Get("/providers/available", func(w http.ResponseWriter, r *http.Request) {
		purpose, ok := parseProviderPurpose(r.URL.Query().Get("purpose"))
		if !ok {
			purpose = llm.PurposeGeneral
		}
		items, err := service.Available(r.Context(), r.URL.Query().Get("scope_id"), purpose)
		if err != nil {
			writeAIError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	})
}

type providerBindingHandler struct{ service providerBindingService }

func (h providerBindingHandler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListBindings(r.Context(), chi.URLParam(r, "scopeID"))
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func parseProviderPurpose(raw string) (llm.Purpose, bool) {
	purpose := llm.Purpose(strings.TrimSpace(raw))
	switch purpose {
	case llm.PurposeGeneral, llm.PurposeDiagnosis, llm.PurposeInspection, llm.PurposeWorkflow:
		return purpose, true
	default:
		return "", false
	}
}

func (h providerBindingHandler) set(w http.ResponseWriter, r *http.Request) {
	purpose, ok := parseProviderPurpose(chi.URLParam(r, "tag"))
	if !ok {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "unsupported provider binding purpose")
		return
	}
	var body providerBindingRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.service.SetBinding(r.Context(), currentUser(r).ID, chi.URLParam(r, "scopeID"), purpose, body.ProviderID)
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h providerBindingHandler) remove(w http.ResponseWriter, r *http.Request) {
	purpose, ok := parseProviderPurpose(chi.URLParam(r, "tag"))
	if !ok {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "unsupported provider binding purpose")
		return
	}
	if err := h.service.RemoveBinding(r.Context(), chi.URLParam(r, "scopeID"), purpose); err != nil {
		writeAIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
