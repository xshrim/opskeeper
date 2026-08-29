package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"opskeeper/backend/authorization"
	"opskeeper/backend/inspection"
)

type inspectionService interface {
	CreatePolicy(context.Context, inspection.Policy, string) (inspection.Policy, error)
	ListPolicies(context.Context, string) ([]inspection.Policy, error)
	StartManualRun(context.Context, string, string, time.Time) (string, error)
	ListRuns(context.Context, string, int) ([]inspection.Run, error)
	ListFindings(context.Context, string, int) ([]inspection.Finding, error)
	CreateChannel(context.Context, inspection.NotificationChannel) (inspection.NotificationChannel, error)
	ListChannels(context.Context, string) ([]inspection.NotificationChannel, error)
	SetPolicyStatus(context.Context, string, string, string) error
}

type inspectionHandler struct{ service inspectionService }
type policyRequest struct {
	ScopeID                string                         `json:"scope_id"`
	Name                   string                         `json:"name"`
	Cron                   string                         `json:"cron"`
	Timezone               string                         `json:"timezone"`
	TargetResourceIDs      []string                       `json:"target_resource_ids"`
	AgentProfileResourceID string                         `json:"agent_profile_resource_id"`
	TargetLabels           map[string]string              `json:"target_labels"`
	TimeoutSeconds         int                            `json:"timeout_seconds"`
	Retries                int                            `json:"retries"`
	MaxConcurrent          int                            `json:"max_concurrent"`
	MaxToolCalls           int                            `json:"max_tool_calls"`
	MaxTokens              int64                          `json:"max_tokens"`
	Maintenance            []inspection.MaintenanceWindow `json:"maintenance"`
}
type channelRequest struct {
	ScopeID            string  `json:"scope_id"`
	Name               string  `json:"name"`
	WebhookURL         string  `json:"webhook_url"`
	Status             string  `json:"status"`
	CredentialID       *string `json:"credential_id"`
	RateLimitPerMinute int     `json:"rate_limit_per_minute"`
}

func registerInspectionRoutes(router chi.Router, service inspectionService, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	if service == nil {
		return
	}
	guard := func(p authorization.Permission) func(http.Handler) http.Handler {
		if requirePermission == nil {
			return func(next http.Handler) http.Handler { return next }
		}
		return requirePermission(p)
	}
	h := inspectionHandler{service}
	router.With(guard(authorization.InspectionManage)).Post("/inspection-policies", h.createPolicy)
	router.With(guard(authorization.InspectionManage)).Get("/inspection-policies", h.listPolicies)
	router.With(guard(authorization.InspectionExecute)).Post("/inspection-policies/{policyID}/runs", h.manualRun)
	router.With(guard(authorization.InspectionManage)).Patch("/inspection-policies/{policyID}/status", h.setPolicyStatus)
	router.With(guard(authorization.InspectionManage)).Get("/inspection-runs", h.listRuns)
	router.With(guard(authorization.InspectionManage)).Get("/inspection-findings", h.listFindings)
	router.With(guard(authorization.InspectionManage)).Post("/notification-channels", h.createChannel)
	router.With(guard(authorization.InspectionManage)).Get("/notification-channels", h.listChannels)
}
func (h inspectionHandler) createPolicy(w http.ResponseWriter, r *http.Request) {
	var body policyRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.service.CreatePolicy(r.Context(), inspection.Policy{ScopeID: body.ScopeID, Name: body.Name, Cron: body.Cron, Timezone: body.Timezone, Status: inspection.PolicyActive, TargetResourceIDs: body.TargetResourceIDs, TargetLabels: body.TargetLabels, AgentProfileResourceID: body.AgentProfileResourceID, Timeout: time.Duration(body.TimeoutSeconds) * time.Second, Retries: body.Retries, MaxConcurrent: body.MaxConcurrent, MaxToolCalls: body.MaxToolCalls, MaxTokens: body.MaxTokens, Maintenance: body.Maintenance}, currentUser(r).ID)
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (h inspectionHandler) listPolicies(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListPolicies(r.Context(), r.URL.Query().Get("scope_id"))
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (h inspectionHandler) manualRun(w http.ResponseWriter, r *http.Request) {
	scopeID := r.URL.Query().Get("scope_id")
	id, err := h.service.StartManualRun(r.Context(), scopeID, chi.URLParam(r, "policyID"), time.Now())
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"run_id": id})
}
func (h inspectionHandler) setPolicyStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ScopeID string `json:"scope_id"`
		Status  string `json:"status"`
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	if err := h.service.SetPolicyStatus(r.Context(), body.ScopeID, chi.URLParam(r, "policyID"), body.Status); err != nil {
		writeInspectionError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h inspectionHandler) listRuns(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListRuns(r.Context(), r.URL.Query().Get("scope_id"), 50)
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (h inspectionHandler) listFindings(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListFindings(r.Context(), r.URL.Query().Get("scope_id"), 100)
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (h inspectionHandler) createChannel(w http.ResponseWriter, r *http.Request) {
	var body channelRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.service.CreateChannel(r.Context(), inspection.NotificationChannel{ScopeID: body.ScopeID, Name: body.Name, WebhookURL: body.WebhookURL, Status: body.Status, CredentialID: body.CredentialID, RateLimitPerMinute: body.RateLimitPerMinute})
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (h inspectionHandler) listChannels(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListChannels(r.Context(), r.URL.Query().Get("scope_id"))
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func writeInspectionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, inspection.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "Inspection item not found")
	case errors.Is(err, inspection.ErrConflict):
		writeError(w, r, http.StatusConflict, "conflict", "Inspection item is not available for this operation")
	case errors.Is(err, authorization.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "forbidden", "You do not have permission for this inspection")
	case inspection.IsInvalid(err):
		writeError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
	default:
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}
