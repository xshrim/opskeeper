package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"opskeeper/backend/audit"
	"opskeeper/backend/authorization"
	"opskeeper/backend/inspection"
	"opskeeper/backend/notification"
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

type notificationAdminService interface {
	ListNotificationProviders(context.Context, string) ([]inspection.NotificationProvider, error)
	CreateConfiguredChannel(context.Context, inspection.NotificationChannel, map[string]string) (inspection.NotificationChannel, error)
	UpdateConfiguredChannel(context.Context, string, string, inspection.NotificationChannel, map[string]string) (inspection.NotificationChannel, error)
	DeleteConfiguredChannel(context.Context, string, string) error
	TestConfiguredChannel(context.Context, string, string) error
}

type inspectionHandler struct {
	service inspectionService
	auditor audit.Logger
}

type notificationCatalogHTTPService interface {
	CreateNotificationTemplate(context.Context, string, string, string, bool, notification.TemplateDraft) (inspection.NotificationTemplateRecord, error)
	ListNotificationTemplates(context.Context, string) ([]inspection.NotificationTemplateRecord, error)
	CreateNotificationTemplateVersion(context.Context, string, string, string, notification.TemplateDraft) (notification.TemplateVersion, error)
	PreviewNotificationTemplate(context.Context, string, string, string, map[string]string) (notification.RenderedTemplate, error)
	PublishNotificationTemplate(context.Context, string, string, string) (notification.TemplateVersion, error)
	DeleteNotificationTemplate(context.Context, string, string) error
	SetNotificationTemplateSharing(context.Context, string, string, bool) error
	CreateNotificationRule(context.Context, string, string, inspection.NotificationRuleRecord) (inspection.NotificationRuleRecord, error)
	ListNotificationRules(context.Context, string) ([]inspection.NotificationRuleRecord, error)
	TestNotificationRule(context.Context, string, string) (int, error)
	UpdateNotificationRule(context.Context, string, string, inspection.NotificationRuleRecord) (inspection.NotificationRuleRecord, error)
	DeleteNotificationRule(context.Context, string, string) error
	SetPolicyNotificationRules(context.Context, string, string, []string) ([]string, error)
	ListPolicyNotificationRules(context.Context, string, string) ([]string, error)
	ListNotificationDeliveries(context.Context, string, string, string, int) ([]inspection.NotificationDeliveryRecord, error)
	RetryNotificationDelivery(context.Context, string, string) error
}
type policyRequest struct {
	ScopeID           string                         `json:"scope_id"`
	Name              string                         `json:"name"`
	Cron              string                         `json:"cron"`
	Timezone          string                         `json:"timezone"`
	TargetResourceIDs []string                       `json:"target_resource_ids"`
	PersonaID         string                         `json:"persona_id"`
	TargetLabels      map[string]string              `json:"target_labels"`
	TimeoutSeconds    int                            `json:"timeout_seconds"`
	Retries           int                            `json:"retries"`
	MaxConcurrent     int                            `json:"max_concurrent"`
	MaxToolCalls      int                            `json:"max_tool_calls"`
	MaxTokens         int64                          `json:"max_tokens"`
	Maintenance       []inspection.MaintenanceWindow `json:"maintenance"`
}
type channelRequest struct {
	ScopeID            string            `json:"scope_id"`
	Name               string            `json:"name"`
	Kind               string            `json:"kind"`
	Config             map[string]string `json:"config"`
	WebhookURL         string            `json:"webhook_url"`
	Status             string            `json:"status"`
	RateLimitPerMinute int               `json:"rate_limit_per_minute"`
	ShareWithChildren  *bool             `json:"share_with_children,omitempty"`
}

type notificationTemplateRequest struct {
	ScopeID           string                          `json:"scope_id"`
	Name              string                          `json:"name"`
	Format            string                          `json:"format"`
	TitleTemplate     string                          `json:"title_template"`
	BodyTemplate      string                          `json:"body_template"`
	PayloadTemplate   json.RawMessage                 `json:"payload_template,omitempty"`
	Variables         []notification.TemplateVariable `json:"variables"`
	ShareWithChildren bool                            `json:"share_with_children"`
}

type notificationRuleRequest struct {
	ScopeID            string                         `json:"scope_id"`
	Name               string                         `json:"name"`
	Status             string                         `json:"status"`
	EventTypes         []notification.EventType       `json:"event_types"`
	MinimumSeverity    string                         `json:"minimum_severity"`
	Filters            notification.RuleFilters       `json:"filters"`
	CooldownSeconds    int                            `json:"cooldown_seconds"`
	AggregationSeconds int                            `json:"aggregation_seconds"`
	MaxBatchSize       int                            `json:"max_batch_size"`
	Silence            notification.SilenceWindow     `json:"silence"`
	Routes             []inspection.NotificationRoute `json:"routes"`
	ShareWithChildren  *bool                          `json:"share_with_children,omitempty"`
}

func registerInspectionRoutes(router chi.Router, service inspectionService, auditor audit.Logger, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	if service == nil {
		return
	}
	guard := func(p authorization.Permission) func(http.Handler) http.Handler {
		if requirePermission == nil {
			return func(next http.Handler) http.Handler { return next }
		}
		return requirePermission(p)
	}
	h := inspectionHandler{service: service, auditor: auditor}
	router.With(guard(authorization.InspectionManage)).Post("/inspection-policies", h.createPolicy)
	router.With(guard(authorization.InspectionManage)).Get("/inspection-policies", h.listPolicies)
	router.With(guard(authorization.InspectionExecute)).Post("/inspection-policies/{policyID}/runs", h.manualRun)
	router.With(guard(authorization.InspectionManage)).Patch("/inspection-policies/{policyID}/status", h.setPolicyStatus)
	router.With(guard(authorization.InspectionManage)).Get("/inspection-runs", h.listRuns)
	router.With(guard(authorization.InspectionManage)).Get("/inspection-findings", h.listFindings)
	if _, ok := service.(notificationAdminService); ok {
		router.With(guard(authorization.NotificationRead)).Get("/notification-providers", h.listNotificationProviders)
		router.With(guard(authorization.NotificationManage)).Post("/notification-channels", h.createConfiguredChannel)
		router.With(guard(authorization.NotificationRead)).Get("/notification-channels", h.listChannels)
		router.With(guard(authorization.NotificationManage)).Patch("/notification-channels/{channelID}", h.updateConfiguredChannel)
		router.With(guard(authorization.NotificationManage)).Delete("/notification-channels/{channelID}", h.deleteConfiguredChannel)
		router.With(guard(authorization.NotificationTest)).Post("/notification-channels/{channelID}/test", h.testConfiguredChannel)
	} else {
		router.With(guard(authorization.InspectionManage)).Post("/notification-channels", h.createChannel)
		router.With(guard(authorization.InspectionManage)).Get("/notification-channels", h.listChannels)
	}
	if _, ok := service.(notificationCatalogHTTPService); ok {
		router.With(guard(authorization.NotificationRead)).Get("/notification-templates", h.listNotificationTemplates)
		router.With(guard(authorization.NotificationManage)).Post("/notification-templates", h.createNotificationTemplate)
		router.With(guard(authorization.NotificationManage)).Delete("/notification-templates/{templateID}", h.deleteNotificationTemplate)
		router.With(guard(authorization.NotificationManage)).Patch("/notification-templates/{templateID}/sharing", h.setNotificationTemplateSharing)
		router.With(guard(authorization.NotificationManage)).Post("/notification-templates/{templateID}/versions", h.createNotificationTemplateVersion)
		router.With(guard(authorization.NotificationRead)).Post("/notification-templates/{templateID}/versions/{versionID}/preview", h.previewNotificationTemplate)
		router.With(guard(authorization.NotificationManage)).Post("/notification-templates/{templateID}/versions/{versionID}/publish", h.publishNotificationTemplate)
		router.With(guard(authorization.NotificationRead)).Get("/notification-rules", h.listNotificationRules)
		router.With(guard(authorization.NotificationManage)).Post("/notification-rules", h.createNotificationRule)
		router.With(guard(authorization.NotificationManage)).Patch("/notification-rules/{ruleID}", h.updateNotificationRule)
		router.With(guard(authorization.NotificationManage)).Delete("/notification-rules/{ruleID}", h.deleteNotificationRule)
		router.With(guard(authorization.NotificationTest)).Post("/notification-rules/{ruleID}/test", h.testNotificationRule)
		router.With(guard(authorization.NotificationRead)).Get("/inspection-policies/{policyID}/notification-rules", h.listPolicyNotificationRules)
		router.With(guard(authorization.InspectionManage)).Put("/inspection-policies/{policyID}/notification-rules", h.setPolicyNotificationRules)
		router.With(guard(authorization.NotificationRead)).Get("/notification-deliveries", h.listNotificationDeliveries)
		router.With(guard(authorization.NotificationRetry)).Post("/notification-deliveries/{deliveryID}/retry", h.retryNotificationDelivery)
	}
}
func (h inspectionHandler) createPolicy(w http.ResponseWriter, r *http.Request) {
	var body policyRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.service.CreatePolicy(r.Context(), inspection.Policy{ScopeID: body.ScopeID, Name: body.Name, Cron: body.Cron, Timezone: body.Timezone, Status: inspection.PolicyActive, TargetResourceIDs: body.TargetResourceIDs, TargetLabels: body.TargetLabels, PersonaID: body.PersonaID, Timeout: time.Duration(body.TimeoutSeconds) * time.Second, Retries: body.Retries, MaxConcurrent: body.MaxConcurrent, MaxToolCalls: body.MaxToolCalls, MaxTokens: body.MaxTokens, Maintenance: body.Maintenance}, currentUser(r).ID)
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
	item, err := h.service.CreateChannel(r.Context(), inspection.NotificationChannel{ScopeID: body.ScopeID, Name: body.Name, WebhookURL: body.WebhookURL, Status: body.Status, RateLimitPerMinute: body.RateLimitPerMinute})
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	h.recordNotificationAudit(r, "notification.channel.create", "notification_channel", item.ID, item.ScopeID)
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

func (h inspectionHandler) listNotificationProviders(w http.ResponseWriter, r *http.Request) {
	service := h.service.(notificationAdminService)
	items, err := service.ListNotificationProviders(r.Context(), r.URL.Query().Get("scope_id"))
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h inspectionHandler) createConfiguredChannel(w http.ResponseWriter, r *http.Request) {
	var body channelRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	if body.Kind == "" && body.WebhookURL != "" {
		body.Kind = "webhook"
		body.Config = map[string]string{"url": body.WebhookURL}
	}
	share := body.ShareWithChildren != nil && *body.ShareWithChildren
	item, err := h.service.(notificationAdminService).CreateConfiguredChannel(r.Context(), inspection.NotificationChannel{ScopeID: body.ScopeID, Name: body.Name, Kind: body.Kind, Status: body.Status, RateLimitPerMinute: body.RateLimitPerMinute, ShareWithChildren: share}, body.Config)
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	h.recordNotificationAudit(r, "notification.channel.create", "notification_channel", item.ID, item.ScopeID)
	writeJSON(w, http.StatusCreated, item)
}

func (h inspectionHandler) updateConfiguredChannel(w http.ResponseWriter, r *http.Request) {
	var body channelRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	share := body.ShareWithChildren != nil && *body.ShareWithChildren
	item, err := h.service.(notificationAdminService).UpdateConfiguredChannel(r.Context(), body.ScopeID, chi.URLParam(r, "channelID"), inspection.NotificationChannel{Name: body.Name, Status: body.Status, RateLimitPerMinute: body.RateLimitPerMinute, ShareWithChildren: share, ShareWithChildrenSet: body.ShareWithChildren != nil}, body.Config)
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	h.recordNotificationAudit(r, "notification.channel.update", "notification_channel", item.ID, item.ScopeID)
	writeJSON(w, http.StatusOK, item)
}

func (h inspectionHandler) deleteConfiguredChannel(w http.ResponseWriter, r *http.Request) {
	scopeID, id := r.URL.Query().Get("scope_id"), chi.URLParam(r, "channelID")
	if err := h.service.(notificationAdminService).DeleteConfiguredChannel(r.Context(), scopeID, id); err != nil {
		writeInspectionError(w, r, err)
		return
	}
	h.recordNotificationAudit(r, "notification.channel.delete", "notification_channel", id, scopeID)
	w.WriteHeader(http.StatusNoContent)
}

func (h inspectionHandler) testConfiguredChannel(w http.ResponseWriter, r *http.Request) {
	scopeID := r.URL.Query().Get("scope_id")
	if err := h.service.(notificationAdminService).TestConfiguredChannel(r.Context(), scopeID, chi.URLParam(r, "channelID")); err != nil {
		writeInspectionError(w, r, err)
		return
	}
	h.recordNotificationAudit(r, "notification.channel.test", "notification_channel", chi.URLParam(r, "channelID"), scopeID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (h inspectionHandler) listNotificationTemplates(w http.ResponseWriter, r *http.Request) {
	scopeID := r.URL.Query().Get("scope_id")
	items, err := h.service.(notificationCatalogHTTPService).ListNotificationTemplates(r.Context(), scopeID)
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h inspectionHandler) createNotificationTemplate(w http.ResponseWriter, r *http.Request) {
	var body notificationTemplateRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.service.(notificationCatalogHTTPService).CreateNotificationTemplate(r.Context(), body.ScopeID, body.Name, currentUser(r).ID, body.ShareWithChildren, body.draft())
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	h.recordNotificationAudit(r, "notification.template.create", "notification_template", item.ID, item.ScopeID)
	writeJSON(w, http.StatusCreated, item)
}

func (h inspectionHandler) createNotificationTemplateVersion(w http.ResponseWriter, r *http.Request) {
	var body notificationTemplateRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	templateID := chi.URLParam(r, "templateID")
	item, err := h.service.(notificationCatalogHTTPService).CreateNotificationTemplateVersion(r.Context(), body.ScopeID, templateID, currentUser(r).ID, body.draft())
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	h.recordNotificationAudit(r, "notification.template.version.create", "notification_template", templateID, body.ScopeID)
	writeJSON(w, http.StatusCreated, item)
}

func (h inspectionHandler) previewNotificationTemplate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ScopeID string            `json:"scope_id"`
		Values  map[string]string `json:"values"`
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.service.(notificationCatalogHTTPService).PreviewNotificationTemplate(r.Context(), body.ScopeID, chi.URLParam(r, "templateID"), chi.URLParam(r, "versionID"), body.Values)
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h inspectionHandler) publishNotificationTemplate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ScopeID string `json:"scope_id"`
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	templateID, versionID := chi.URLParam(r, "templateID"), chi.URLParam(r, "versionID")
	item, err := h.service.(notificationCatalogHTTPService).PublishNotificationTemplate(r.Context(), body.ScopeID, templateID, versionID)
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	h.recordNotificationAudit(r, "notification.template.publish", "notification_template_version", versionID, body.ScopeID)
	writeJSON(w, http.StatusOK, item)
}

func (h inspectionHandler) deleteNotificationTemplate(w http.ResponseWriter, r *http.Request) {
	scopeID, id := r.URL.Query().Get("scope_id"), chi.URLParam(r, "templateID")
	if err := h.service.(notificationCatalogHTTPService).DeleteNotificationTemplate(r.Context(), scopeID, id); err != nil {
		writeInspectionError(w, r, err)
		return
	}
	h.recordNotificationAudit(r, "notification.template.delete", "notification_template", id, scopeID)
	w.WriteHeader(http.StatusNoContent)
}

func (h inspectionHandler) setNotificationTemplateSharing(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ScopeID           string `json:"scope_id"`
		ShareWithChildren bool   `json:"share_with_children"`
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	id := chi.URLParam(r, "templateID")
	if err := h.service.(notificationCatalogHTTPService).SetNotificationTemplateSharing(r.Context(), body.ScopeID, id, body.ShareWithChildren); err != nil {
		writeInspectionError(w, r, err)
		return
	}
	h.recordNotificationAudit(r, "notification.template.sharing.update", "notification_template", id, body.ScopeID)
	w.WriteHeader(http.StatusNoContent)
}

func (h inspectionHandler) listNotificationRules(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.(notificationCatalogHTTPService).ListNotificationRules(r.Context(), r.URL.Query().Get("scope_id"))
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h inspectionHandler) createNotificationRule(w http.ResponseWriter, r *http.Request) {
	var body notificationRuleRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.service.(notificationCatalogHTTPService).CreateNotificationRule(r.Context(), body.ScopeID, currentUser(r).ID, body.record())
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	h.recordNotificationAudit(r, "notification.rule.create", "notification_rule", item.ID, body.ScopeID)
	writeJSON(w, http.StatusCreated, item)
}

func (h inspectionHandler) updateNotificationRule(w http.ResponseWriter, r *http.Request) {
	var body notificationRuleRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	id := chi.URLParam(r, "ruleID")
	item, err := h.service.(notificationCatalogHTTPService).UpdateNotificationRule(r.Context(), body.ScopeID, id, body.record())
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	h.recordNotificationAudit(r, "notification.rule.update", "notification_rule", id, body.ScopeID)
	writeJSON(w, http.StatusOK, item)
}

func (h inspectionHandler) deleteNotificationRule(w http.ResponseWriter, r *http.Request) {
	scopeID, id := r.URL.Query().Get("scope_id"), chi.URLParam(r, "ruleID")
	if err := h.service.(notificationCatalogHTTPService).DeleteNotificationRule(r.Context(), scopeID, id); err != nil {
		writeInspectionError(w, r, err)
		return
	}
	h.recordNotificationAudit(r, "notification.rule.disable", "notification_rule", id, scopeID)
	w.WriteHeader(http.StatusNoContent)
}

func (h inspectionHandler) testNotificationRule(w http.ResponseWriter, r *http.Request) {
	scopeID, id := r.URL.Query().Get("scope_id"), chi.URLParam(r, "ruleID")
	count, err := h.service.(notificationCatalogHTTPService).TestNotificationRule(r.Context(), scopeID, id)
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	h.recordNotificationAudit(r, "notification.rule.test", "notification_rule", id, scopeID)
	writeJSON(w, http.StatusOK, map[string]int{"routes_tested": count})
}

func (h inspectionHandler) listPolicyNotificationRules(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.(notificationCatalogHTTPService).ListPolicyNotificationRules(r.Context(), r.URL.Query().Get("scope_id"), chi.URLParam(r, "policyID"))
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]string{"rule_ids": items})
}

func (h inspectionHandler) setPolicyNotificationRules(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ScopeID string   `json:"scope_id"`
		RuleIDs []string `json:"rule_ids"`
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	policyID := chi.URLParam(r, "policyID")
	items, err := h.service.(notificationCatalogHTTPService).SetPolicyNotificationRules(r.Context(), body.ScopeID, policyID, body.RuleIDs)
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	h.recordNotificationAudit(r, "notification.policy_rules.update", "inspection_policy", policyID, body.ScopeID)
	writeJSON(w, http.StatusOK, map[string][]string{"rule_ids": items})
}

func (h inspectionHandler) listNotificationDeliveries(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.service.(notificationCatalogHTTPService).ListNotificationDeliveries(r.Context(), r.URL.Query().Get("scope_id"), r.URL.Query().Get("status"), r.URL.Query().Get("event_type"), limit)
	if err != nil {
		writeInspectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h inspectionHandler) retryNotificationDelivery(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ScopeID string `json:"scope_id"`
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	id := chi.URLParam(r, "deliveryID")
	if err := h.service.(notificationCatalogHTTPService).RetryNotificationDelivery(r.Context(), body.ScopeID, id); err != nil {
		writeInspectionError(w, r, err)
		return
	}
	h.recordNotificationAudit(r, "notification.delivery.retry", "notification_delivery", id, body.ScopeID)
	w.WriteHeader(http.StatusAccepted)
}

func (request notificationTemplateRequest) draft() notification.TemplateDraft {
	return notification.TemplateDraft{Format: request.Format, TitleTemplate: request.TitleTemplate, BodyTemplate: request.BodyTemplate, PayloadTemplate: request.PayloadTemplate, Variables: request.Variables}
}

func (request notificationRuleRequest) record() inspection.NotificationRuleRecord {
	share := request.ShareWithChildren != nil && *request.ShareWithChildren
	return inspection.NotificationRuleRecord{
		Rule:                 notification.Rule{Name: request.Name, Status: request.Status, EventTypes: request.EventTypes, MinimumSeverity: request.MinimumSeverity, Filters: request.Filters, Cooldown: time.Duration(request.CooldownSeconds) * time.Second, Aggregation: time.Duration(request.AggregationSeconds) * time.Second, MaxBatchSize: request.MaxBatchSize, Silence: request.Silence},
		Routes:               request.Routes,
		ShareWithChildren:    share,
		ShareWithChildrenSet: request.ShareWithChildren != nil,
	}
}

func (h inspectionHandler) recordNotificationAudit(r *http.Request, action, targetType, targetID, scopeID string) {
	if h.auditor == nil {
		return
	}
	_ = h.auditor.Record(r.Context(), audit.Event{ActorUserID: currentUser(r).ID, Action: action, TargetType: targetType, TargetID: targetID, ScopeID: scopeID, Result: "success", RequestID: middleware.GetReqID(r.Context()), ClientIP: requestClientIP(r)})
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
