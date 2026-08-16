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
	"opskeeper/backend/operation"
)

type operationService interface {
	Request(context.Context, operation.Request) (operation.Request, error)
	Get(context.Context, string) (operation.Request, error)
	List(context.Context, string, int) ([]operation.Request, error)
	Approve(context.Context, operation.Approval) (operation.Request, error)
	Start(context.Context, string, string) (string, error)
	GetExecution(context.Context, string) (operation.Execution, error)
	CreatePolicy(context.Context, operation.Policy) (operation.Policy, error)
	ListPolicies(context.Context, string) ([]operation.Policy, error)
}

type operationHandler struct {
	service operationService
	auditor audit.Logger
}

type operationRequestBody struct {
	ScopeID          string         `json:"scope_id"`
	TargetResourceID string         `json:"target_resource_id"`
	OperationName    string         `json:"operation_name"`
	RiskLevel        string         `json:"risk_level"`
	Parameters       map[string]any `json:"parameters"`
	ImpactSummary    string         `json:"impact_summary"`
	RollbackSummary  string         `json:"rollback_summary"`
	DryRun           map[string]any `json:"dry_run"`
	IdempotencyKey   string         `json:"idempotency_key"`
	Source           string         `json:"source"`
}
type approvalBody struct {
	Decision       string `json:"decision"`
	ParametersHash string `json:"parameters_hash"`
	Comment        string `json:"comment"`
}
type startOperationBody struct {
	IdempotencyKey string `json:"idempotency_key"`
}
type operationPolicyBody struct {
	ScopeID             string   `json:"scope_id"`
	Name                string   `json:"name"`
	TargetKinds         []string `json:"target_kinds"`
	OperationNames      []string `json:"operation_names"`
	MinimumRisk         string   `json:"minimum_risk"`
	ApprovalRequired    bool     `json:"approval_required"`
	ExpiresAfterSeconds int      `json:"expires_after_seconds"`
}

func registerOperationRoutes(router chi.Router, service operationService, auditor audit.Logger, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	if service == nil {
		return
	}
	guard := func(permission authorization.Permission) func(http.Handler) http.Handler {
		if requirePermission == nil {
			return func(next http.Handler) http.Handler { return next }
		}
		return requirePermission(permission)
	}
	h := operationHandler{service: service, auditor: auditor}
	router.With(guard(authorization.ResourceUse)).Post("/operation-requests", h.create)
	router.With(guard(authorization.ResourceUse)).Get("/operation-requests", h.list)
	router.With(guard(authorization.ResourceUse)).Get("/operation-requests/{requestID}", h.get)
	router.With(guard(authorization.OperationApprove)).Post("/operation-requests/{requestID}/approvals", h.approve)
	router.With(guard(authorization.ResourceUse)).Post("/operation-requests/{requestID}/execute", h.start)
	router.With(guard(authorization.ResourceUse)).Get("/operation-executions/{executionID}", h.getExecution)
	router.With(guard(authorization.OperationApprove)).Post("/operation-policies", h.createPolicy)
	router.With(guard(authorization.OperationApprove)).Get("/operation-policies", h.listPolicies)
}

func (h operationHandler) create(w http.ResponseWriter, r *http.Request) {
	var body operationRequestBody
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.service.Request(r.Context(), operation.Request{ScopeID: body.ScopeID, TargetResourceID: body.TargetResourceID, RequestedBy: currentUser(r).ID, Source: body.Source, OperationName: body.OperationName, RiskLevel: body.RiskLevel, Parameters: body.Parameters, ImpactSummary: body.ImpactSummary, RollbackSummary: body.RollbackSummary, DryRun: body.DryRun, IdempotencyKey: body.IdempotencyKey})
	if err != nil {
		writeOperationError(w, r, err)
		return
	}
	h.record(r, "operation.request.create", "operation_request", item.ID, item.ScopeID, map[string]any{"operation": item.OperationName, "risk": item.RiskLevel, "dry_run": item.DryRun})
	writeJSON(w, http.StatusCreated, item)
}
func (h operationHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.service.List(r.Context(), r.URL.Query().Get("scope_id"), limit)
	if err != nil {
		writeOperationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (h operationHandler) get(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.Get(r.Context(), chi.URLParam(r, "requestID"))
	if err != nil {
		writeOperationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (h operationHandler) approve(w http.ResponseWriter, r *http.Request) {
	var body approvalBody
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.service.Approve(r.Context(), operation.Approval{OperationRequestID: chi.URLParam(r, "requestID"), ApproverUserID: currentUser(r).ID, Decision: body.Decision, ParametersHash: body.ParametersHash, Comment: body.Comment})
	if err != nil {
		writeOperationError(w, r, err)
		return
	}
	h.record(r, "operation.request."+body.Decision, "operation_request", item.ID, item.ScopeID, map[string]any{"parameters_hash": item.ParametersHash})
	writeJSON(w, http.StatusOK, item)
}
func (h operationHandler) start(w http.ResponseWriter, r *http.Request) {
	var body startOperationBody
	if !decodeRequest(w, r, &body) {
		return
	}
	id, err := h.service.Start(r.Context(), chi.URLParam(r, "requestID"), body.IdempotencyKey)
	if err != nil {
		writeOperationError(w, r, err)
		return
	}
	item, err := h.service.GetExecution(r.Context(), id)
	if err != nil {
		writeOperationError(w, r, err)
		return
	}
	requestItem, _ := h.service.Get(r.Context(), chi.URLParam(r, "requestID"))
	h.record(r, "operation.execution.start", "operation_execution", id, requestItem.ScopeID, map[string]any{"idempotency_key": body.IdempotencyKey, "operation_request_id": item.OperationRequestID})
	writeJSON(w, http.StatusAccepted, item)
}
func (h operationHandler) getExecution(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.GetExecution(r.Context(), chi.URLParam(r, "executionID"))
	if err != nil {
		writeOperationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (h operationHandler) createPolicy(w http.ResponseWriter, r *http.Request) {
	var body operationPolicyBody
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.service.CreatePolicy(r.Context(), operation.Policy{ScopeID: body.ScopeID, Name: body.Name, TargetKinds: body.TargetKinds, OperationNames: body.OperationNames, MinimumRisk: body.MinimumRisk, ApprovalRequired: body.ApprovalRequired, ExpiresAfterSeconds: body.ExpiresAfterSeconds})
	if err != nil {
		writeOperationError(w, r, err)
		return
	}
	h.record(r, "operation.policy.create", "operation_policy", item.ID, item.ScopeID, map[string]any{"minimum_risk": item.MinimumRisk, "approval_required": item.ApprovalRequired})
	writeJSON(w, http.StatusCreated, item)
}
func (h operationHandler) listPolicies(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListPolicies(r.Context(), r.URL.Query().Get("scope_id"))
	if err != nil {
		writeOperationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (h operationHandler) record(r *http.Request, action, targetType, targetID, scopeID string, details map[string]any) {
	if h.auditor != nil {
		_ = h.auditor.Record(r.Context(), audit.Event{ActorUserID: currentUser(r).ID, Action: action, TargetType: targetType, TargetID: targetID, ScopeID: scopeID, RequestID: middleware.GetReqID(r.Context()), ClientIP: requestClientIP(r), Details: details})
	}
}
func writeOperationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, authorization.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "forbidden", "You do not have permission for this operation")
	case errors.Is(err, operation.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "Operation request not found")
	case errors.Is(err, operation.ErrConflict), errors.Is(err, operation.ErrApprovalRequired), errors.Is(err, operation.ErrApprovalInvalid):
		writeError(w, r, http.StatusConflict, "operation_unavailable", "Operation is not available for execution")
	default:
		var syntax *json.SyntaxError
		if errors.As(err, &syntax) || errors.Is(err, operation.ErrApprovalInvalid) {
			writeError(w, r, http.StatusBadRequest, "invalid_request", "Invalid operation request")
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			writeError(w, r, http.StatusGatewayTimeout, "timeout", "Operation timed out")
			return
		}
		writeError(w, r, http.StatusBadRequest, "invalid_request", "Invalid operation request")
	}
}
