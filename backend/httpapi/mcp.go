package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"opskeeper/backend/audit"
	"opskeeper/backend/authorization"
	"opskeeper/backend/mcp"
	"opskeeper/backend/resource"
)

type mcpService interface {
	Discover(context.Context, string) (mcp.Snapshot, error)
	ListSnapshots(context.Context, string, int) ([]mcp.Snapshot, error)
	Call(context.Context, string, string, map[string]any) (json.RawMessage, error)
}
type mcpHandler struct {
	service mcpService
	auditor audit.Logger
}
type callMCPToolBody struct {
	Arguments map[string]any `json:"arguments"`
}

func registerMCPRoutes(router chi.Router, service mcpService, auditor audit.Logger, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	if service == nil {
		return
	}
	guard := func(p authorization.Permission) func(http.Handler) http.Handler {
		if requirePermission == nil {
			return func(n http.Handler) http.Handler { return n }
		}
		return requirePermission(p)
	}
	h := mcpHandler{service, auditor}
	router.With(guard(authorization.ResourceUse)).Post("/mcp-servers/{resourceID}/discover", h.discover)
	router.With(guard(authorization.ResourceRead)).Get("/mcp-servers/{resourceID}/snapshots", h.snapshots)
	router.With(guard(authorization.ResourceUse)).Post("/mcp-servers/{resourceID}/tools/{toolName}", h.call)
}
func (h mcpHandler) discover(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.Discover(r.Context(), chi.URLParam(r, "resourceID"))
	if err != nil {
		writeMCPError(w, r, err)
		return
	}
	h.record(r, "mcp.discover", item.ServerResourceID, item.ScopeID, map[string]any{"status": item.Status, "content_hash": item.Hash})
	writeJSON(w, http.StatusOK, item)
}
func (h mcpHandler) snapshots(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.service.ListSnapshots(r.Context(), chi.URLParam(r, "resourceID"), limit)
	if err != nil {
		writeMCPError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (h mcpHandler) call(w http.ResponseWriter, r *http.Request) {
	var body callMCPToolBody
	if !decodeRequest(w, r, &body) {
		return
	}
	raw, err := h.service.Call(r.Context(), chi.URLParam(r, "resourceID"), chi.URLParam(r, "toolName"), body.Arguments)
	if err != nil {
		writeMCPError(w, r, err)
		return
	}
	h.record(r, "mcp.tool.call", chi.URLParam(r, "resourceID"), "", map[string]any{"tool": chi.URLParam(r, "toolName"), "untrusted": true})
	writeJSON(w, http.StatusOK, map[string]any{"untrusted": true, "content": json.RawMessage(raw)})
}
func (h mcpHandler) record(r *http.Request, action, targetID, scopeID string, details map[string]any) {
	if h.auditor != nil {
		_ = h.auditor.Record(r.Context(), audit.Event{ActorUserID: currentUser(r).ID, Action: action, TargetType: "mcp_server", TargetID: targetID, ScopeID: scopeID, RequestID: middleware.GetReqID(r.Context()), ClientIP: requestClientIP(r), Details: details})
	}
}
func writeMCPError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, authorization.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "forbidden", "You do not have permission for this MCP server")
	case errors.Is(err, resource.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "MCP server resource not found")
	case errors.Is(err, mcp.ErrInvalid):
		writeError(w, r, http.StatusBadRequest, "invalid_request", "Invalid MCP server configuration")
	default:
		writeError(w, r, http.StatusBadGateway, "mcp_unavailable", "MCP server is unavailable")
	}
}
