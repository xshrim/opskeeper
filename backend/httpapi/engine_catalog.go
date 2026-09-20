package httpapi

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"opskeeper/backend/authorization"
	"opskeeper/backend/engine"
)

type engineCatalogService interface {
	List(context.Context, string) ([]engine.CatalogEngine, error)
	Get(context.Context, string) (engine.CatalogEngine, error)
	Create(context.Context, engine.EngineInput) (engine.CatalogEngine, error)
	Update(context.Context, string, engine.EnginePatch) (engine.CatalogEngine, error)
	ListCapabilityTerms(context.Context) ([]string, error)
	AddCapabilityTerm(context.Context, string, string) (string, error)
	ListBindings(context.Context, string) ([]engine.Binding, error)
	SetBinding(context.Context, string, string, string, string) (engine.Binding, error)
	RemoveBinding(context.Context, string, string, string) error
}

type createEngineRequest struct {
	ScopeID     string         `json:"scope_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Icon        string         `json:"icon"`
	Config      map[string]any `json:"config"`
	Status      string         `json:"status"`
}
type updateEngineRequest struct {
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	Icon        *string         `json:"icon"`
	Config      *map[string]any `json:"config"`
	Status      *string         `json:"status"`
}
type engineBindingRequest struct {
	EngineID string `json:"engine_id"`
}

type engineCapabilityTermRequest struct {
	Term string `json:"term"`
}

func registerEngineCatalogRoutes(router chi.Router, service engineCatalogService, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	if service == nil {
		return
	}
	guard := func(permission authorization.Permission) func(http.Handler) http.Handler {
		if requirePermission == nil {
			return func(next http.Handler) http.Handler { return next }
		}
		return requirePermission(permission)
	}
	router.With(guard(authorization.EngineRead)).Get("/engines", func(w http.ResponseWriter, r *http.Request) {
		items, err := service.List(r.Context(), r.URL.Query().Get("scope_id"))
		if err != nil {
			writeAIError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})
	router.With(guard(authorization.EngineRead)).Get("/engines/{engineID}", func(w http.ResponseWriter, r *http.Request) {
		item, err := service.Get(r.Context(), chi.URLParam(r, "engineID"))
		if err != nil {
			writeAIError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	router.With(guard(authorization.EngineManage)).Post("/engines", func(w http.ResponseWriter, r *http.Request) {
		var body createEngineRequest
		if !decodeRequest(w, r, &body) {
			return
		}
		item, err := service.Create(r.Context(), engine.EngineInput{ScopeID: body.ScopeID, Name: body.Name, Description: body.Description, Icon: body.Icon, Config: body.Config, Status: body.Status})
		if err != nil {
			writeAIError(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	})
	router.With(guard(authorization.EngineManage)).Patch("/engines/{engineID}", func(w http.ResponseWriter, r *http.Request) {
		var body updateEngineRequest
		if !decodeRequest(w, r, &body) {
			return
		}
		item, err := service.Update(r.Context(), chi.URLParam(r, "engineID"), engine.EnginePatch{Name: body.Name, Description: body.Description, Icon: body.Icon, Config: body.Config, Status: body.Status})
		if err != nil {
			writeAIError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	router.With(guard(authorization.EngineRead)).Get("/engine-capability-terms", func(w http.ResponseWriter, r *http.Request) {
		terms, err := service.ListCapabilityTerms(r.Context())
		if err != nil {
			writeAIError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": terms})
	})
	router.With(guard(authorization.EngineManage)).Post("/engine-capability-terms", func(w http.ResponseWriter, r *http.Request) {
		var body engineCapabilityTermRequest
		if !decodeRequest(w, r, &body) {
			return
		}
		term, err := service.AddCapabilityTerm(r.Context(), currentUser(r).ID, body.Term)
		if err != nil {
			writeAIError(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"term": term})
	})
	router.With(guard(authorization.EngineRead)).Get("/scopes/{scopeID}/engine-bindings", func(w http.ResponseWriter, r *http.Request) {
		items, err := service.ListBindings(r.Context(), chi.URLParam(r, "scopeID"))
		if err != nil {
			writeAIError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	})
	router.With(guard(authorization.EngineManage)).Put("/scopes/{scopeID}/engine-bindings/{tag}", func(w http.ResponseWriter, r *http.Request) {
		var body engineBindingRequest
		if !decodeRequest(w, r, &body) {
			return
		}
		item, err := service.SetBinding(r.Context(), currentUser(r).ID, chi.URLParam(r, "scopeID"), chi.URLParam(r, "tag"), body.EngineID)
		if err != nil {
			writeAIError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	router.With(guard(authorization.EngineManage)).Delete("/scopes/{scopeID}/engine-bindings/{tag}/{engineID}", func(w http.ResponseWriter, r *http.Request) {
		if err := service.RemoveBinding(r.Context(), chi.URLParam(r, "scopeID"), chi.URLParam(r, "tag"), chi.URLParam(r, "engineID")); err != nil {
			writeAIError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
