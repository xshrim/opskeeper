package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"opskeeper/backend/aiengine"
	"opskeeper/backend/authorization"
	"opskeeper/backend/resource"
)

type workflowRunHandler struct {
	resources resourceService
	runs      aiengine.WorkflowRunStore
	executor  workflowExecutionService
	events    aiengine.EventStore
}

type workflowExecutionService interface {
	Execute(context.Context, aiengine.Workflow, string) (aiengine.WorkflowRun, error)
}

type createWorkflowRunRequest struct {
	ExecutionID string         `json:"execution_id,omitempty"`
	Input       map[string]any `json:"input,omitempty"`
}

type updateWorkflowRunRequest struct {
	CurrentNodeID string         `json:"current_node_id,omitempty"`
	Attempt       int            `json:"attempt,omitempty"`
	State         map[string]any `json:"state,omitempty"`
	ErrorCode     string         `json:"error_code,omitempty"`
	ErrorMessage  string         `json:"error_message,omitempty"`
}

func registerWorkflowRoutes(router chi.Router, resources resourceService, runs aiengine.WorkflowRunStore, executor workflowExecutionService, events aiengine.EventStore, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	if resources == nil || runs == nil {
		return
	}
	guard := func(permission authorization.Permission) func(http.Handler) http.Handler {
		if requirePermission == nil {
			return func(next http.Handler) http.Handler { return next }
		}
		return requirePermission(permission)
	}
	h := workflowRunHandler{resources: resources, runs: runs, executor: executor, events: events}
	router.With(guard(authorization.DiagnosisStart)).Post("/workflows/{workflowID}/runs", h.create)
	router.With(guard(authorization.DiagnosisRead)).Get("/workflows/{workflowID}/runs", h.list)
	router.With(guard(authorization.DiagnosisRead)).Get("/workflow-runs/{runID}", h.get)
	router.With(guard(authorization.DiagnosisRead)).Get("/workflow-runs/{runID}/events", h.eventsForRun)
	router.With(guard(authorization.DiagnosisStart)).Post("/workflow-runs/{runID}/start", h.start)
	router.With(guard(authorization.DiagnosisStart)).Post("/workflow-runs/{runID}/resume", h.resume)
	router.With(guard(authorization.DiagnosisStart)).Post("/workflow-runs/{runID}/cancel", h.cancel)
	router.With(guard(authorization.DiagnosisStart)).Patch("/workflow-runs/{runID}", h.update)
}

func (h workflowRunHandler) eventsForRun(w http.ResponseWriter, r *http.Request) {
	if h.events == nil {
		writeError(w, r, http.StatusNotImplemented, "not_configured", "workflow event store is not configured")
		return
	}
	run, err := h.runs.GetWorkflowRun(r.Context(), chi.URLParam(r, "runID"))
	if err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	if _, err := h.workflow(r.Context(), run.WorkflowID); err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	// Use the common AI execution SSE implementation so cursor parsing,
	// flushing, terminal events and Last-Event-ID remain identical.
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("executionID", run.ExecutionID)
	request := r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, ctx))
	aiEngineEventHandler{store: h.events}.events(w, request)
}

func (h workflowRunHandler) workflow(ctx context.Context, id string) (aiengine.Workflow, error) {
	item, err := h.resources.Get(ctx, strings.TrimSpace(id))
	if err != nil {
		return aiengine.Workflow{}, err
	}
	if item.Kind != "Workflow" || !resourceAllowed(ctx, item) {
		return aiengine.Workflow{}, authorization.ErrForbidden
	}
	return aiengine.WorkflowFromResource(item.ID, item.ScopeID, item.Name, item.Config)
}

func resourceAllowed(ctx context.Context, item resource.Resource) bool {
	if filter, restricted := authorization.ResourceFilterFromContext(ctx); restricted {
		return filter.Allows(item.ScopeID, item.ID)
	}
	if filter, restricted := authorization.ScopeFilterFromContext(ctx); restricted {
		return filter.Allows(item.ScopeID)
	}
	return true
}

func (h workflowRunHandler) create(w http.ResponseWriter, r *http.Request) {
	wf, err := h.workflow(r.Context(), chi.URLParam(r, "workflowID"))
	if err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	if !wf.Enabled {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "workflow is disabled")
		return
	}
	var body createWorkflowRunRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	executionID := strings.TrimSpace(body.ExecutionID)
	if executionID == "" {
		executionID = "workflow-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	run, err := h.runs.CreateWorkflowRun(r.Context(), aiengine.WorkflowRunInput{WorkflowID: wf.ID, WorkflowVersion: wf.Version, ExecutionID: executionID, ScopeID: wf.ScopeID, CreatedBy: currentUser(r).ID, Input: body.Input})
	if err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

func (h workflowRunHandler) list(w http.ResponseWriter, r *http.Request) {
	if _, err := h.workflow(r.Context(), chi.URLParam(r, "workflowID")); err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.runs.ListWorkflowRuns(r.Context(), chi.URLParam(r, "workflowID"), limit)
	if err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h workflowRunHandler) get(w http.ResponseWriter, r *http.Request) {
	run, err := h.runs.GetWorkflowRun(r.Context(), chi.URLParam(r, "runID"))
	if err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	if _, err := h.workflow(r.Context(), run.WorkflowID); err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h workflowRunHandler) transition(w http.ResponseWriter, r *http.Request, status aiengine.WorkflowRunStatus) {
	run, err := h.runs.GetWorkflowRun(r.Context(), chi.URLParam(r, "runID"))
	if err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	if _, err := h.workflow(r.Context(), run.WorkflowID); err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	updated, err := h.runs.UpdateWorkflowRun(r.Context(), run.ID, aiengine.WorkflowRunPatch{Status: status, Attempt: run.Attempt + 1})
	if err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h workflowRunHandler) start(w http.ResponseWriter, r *http.Request) {
	h.execute(w, r, false)
}
func (h workflowRunHandler) resume(w http.ResponseWriter, r *http.Request) {
	run, err := h.runs.GetWorkflowRun(r.Context(), chi.URLParam(r, "runID"))
	if err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	if _, err := h.workflow(r.Context(), run.WorkflowID); err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	if run.Status != aiengine.WorkflowRunWaitingApproval {
		writeError(w, r, http.StatusConflict, "conflict", "Workflow run is not waiting for approval")
		return
	}
	h.executeRun(w, r, run)
}

func (h workflowRunHandler) execute(w http.ResponseWriter, r *http.Request, _ bool) {
	run, err := h.runs.GetWorkflowRun(r.Context(), chi.URLParam(r, "runID"))
	if err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	h.executeRun(w, r, run)
}

func (h workflowRunHandler) executeRun(w http.ResponseWriter, r *http.Request, run aiengine.WorkflowRun) {
	wf, err := h.workflow(r.Context(), run.WorkflowID)
	if err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	if h.executor == nil {
		writeError(w, r, http.StatusNotImplemented, "not_configured", "workflow executor is not configured")
		return
	}
	updated, err := h.executor.Execute(r.Context(), wf, run.ID)
	if err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
func (h workflowRunHandler) cancel(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, aiengine.WorkflowRunCancelled)
}

func (h workflowRunHandler) transitionRun(w http.ResponseWriter, r *http.Request, run aiengine.WorkflowRun, status aiengine.WorkflowRunStatus) {
	updated, err := h.runs.UpdateWorkflowRun(r.Context(), run.ID, aiengine.WorkflowRunPatch{Status: status, Attempt: run.Attempt + 1})
	if err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h workflowRunHandler) update(w http.ResponseWriter, r *http.Request) {
	run, err := h.runs.GetWorkflowRun(r.Context(), chi.URLParam(r, "runID"))
	if err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	if _, err := h.workflow(r.Context(), run.WorkflowID); err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	var body updateWorkflowRunRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	if body.Attempt < 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "attempt must not be negative")
		return
	}
	updated, err := h.runs.UpdateWorkflowRun(r.Context(), run.ID, aiengine.WorkflowRunPatch{Status: run.Status, CurrentNodeID: body.CurrentNodeID, Attempt: body.Attempt, State: body.State, ErrorCode: body.ErrorCode, ErrorMessage: body.ErrorMessage})
	if err != nil {
		writeWorkflowError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func writeWorkflowError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, aiengine.ErrWorkflowRunNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "Workflow run not found")
	case errors.Is(err, aiengine.ErrWorkflowRunConflict):
		writeError(w, r, http.StatusConflict, "conflict", "Workflow run conflicts with existing data")
	case errors.Is(err, aiengine.ErrWorkflowInvalid):
		writeError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, aiengine.ErrWorkflowCycle):
		writeError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, authorization.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "forbidden", "You do not have permission for this workflow")
	default:
		writeAIEngineError(w, r, err)
	}
}
