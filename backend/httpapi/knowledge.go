package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"opskeeper/backend/aiengine"
	"opskeeper/backend/authorization"
)

type knowledgeHandler struct{ resources resourceService }

type knowledgeSearchRequest struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k,omitempty"`
}

func registerKnowledgeRoutes(router chi.Router, resources resourceService, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	if resources == nil {
		return
	}
	guard := func(next http.Handler) http.Handler {
		if requirePermission == nil {
			return next
		}
		return requirePermission(authorization.ResourceUse)(next)
	}
	router.With(guard).Post("/knowledge-bases/{knowledgeBaseID}/search", knowledgeHandler{resources: resources}.search)
}

func (h knowledgeHandler) search(w http.ResponseWriter, r *http.Request) {
	item, err := h.resources.Get(r.Context(), chi.URLParam(r, "knowledgeBaseID"))
	if err != nil {
		writeAIEngineError(w, r, err)
		return
	}
	if item.Kind != "KnowledgeBase" || !resourceAllowed(r.Context(), item) {
		writeError(w, r, http.StatusForbidden, "forbidden", "You do not have permission for this knowledge base")
		return
	}
	var body knowledgeSearchRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	var base aiengine.KnowledgeBase
	raw, marshalErr := json.Marshal(item.Config)
	if marshalErr != nil || json.Unmarshal(raw, &base) != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "KnowledgeBase config is invalid")
		return
	}
	result, err := aiengine.SearchDocuments(aiengine.KnowledgeQuery{ScopeID: item.ScopeID, KnowledgeBaseID: item.ID, Query: strings.TrimSpace(body.Query), TopK: body.TopK}, base)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
