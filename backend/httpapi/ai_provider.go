package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"opskeeper/backend/authorization"
	"opskeeper/backend/llm"
)

type aiProviderBindingService interface {
	ListBindings(context.Context, string) ([]llm.ScopeProviderBinding, error)
	SetBinding(context.Context, string, string, llm.Purpose, string) (llm.ScopeProviderBinding, error)
	RemoveBinding(context.Context, string, llm.Purpose) error
}
type aiProviderAvailabilityService interface {
	Available(context.Context, string, llm.Purpose) ([]llm.AvailableProvider, error)
}

type aiProviderBindingRequest struct {
	ProviderResourceID string `json:"provider_resource_id"`
}

func registerAIProviderBindingRoutes(router chi.Router, service aiProviderBindingService, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	if service == nil {
		return
	}
	guard := func(permission authorization.Permission) func(http.Handler) http.Handler {
		if requirePermission == nil {
			return func(next http.Handler) http.Handler { return next }
		}
		return requirePermission(permission)
	}
	h := aiProviderBindingHandler{service: service}
	router.With(guard(authorization.ResourceRead)).Get("/scopes/{scopeID}/ai-provider-bindings", h.list)
	router.With(guard(authorization.ResourceUpdate)).Put("/scopes/{scopeID}/ai-provider-bindings/{tag}", h.set)
	router.With(guard(authorization.ResourceUpdate)).Delete("/scopes/{scopeID}/ai-provider-bindings/{tag}", h.remove)
}

func registerAIProviderAvailabilityRoute(router chi.Router, service aiProviderAvailabilityService, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	if service == nil {
		return
	}
	guard := func(next http.Handler) http.Handler {
		if requirePermission == nil {
			return next
		}
		return requirePermission(authorization.ResourceUse)(next)
	}
	router.With(guard).Get("/ai-providers/available", func(w http.ResponseWriter, r *http.Request) {
		purpose, ok := parseProviderPurpose(r.URL.Query().Get("purpose"))
		if !ok {
			purpose = llm.PurposeDefault
		}
		items, err := service.Available(r.Context(), r.URL.Query().Get("scope_id"), purpose)
		if err != nil {
			writeAIError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	})
}

type aiProviderBindingHandler struct{ service aiProviderBindingService }

func (h aiProviderBindingHandler) list(w http.ResponseWriter, r *http.Request) {
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
	case llm.PurposeDefault, llm.PurposeDiagnosis, llm.PurposeInspection, llm.PurposeWorkflow:
		return purpose, true
	default:
		return "", false
	}
}

func (h aiProviderBindingHandler) set(w http.ResponseWriter, r *http.Request) {
	purpose, ok := parseProviderPurpose(chi.URLParam(r, "tag"))
	if !ok {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "unsupported provider binding purpose")
		return
	}
	var body aiProviderBindingRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.service.SetBinding(r.Context(), currentUser(r).ID, chi.URLParam(r, "scopeID"), purpose, body.ProviderResourceID)
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h aiProviderBindingHandler) remove(w http.ResponseWriter, r *http.Request) {
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
