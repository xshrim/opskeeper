package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"opskeeper/backend/aiengine"
	"opskeeper/backend/authorization"
)

type aiEngineHandler struct{ engine aiengine.Engine }

type executeAIEngineRequest struct {
	ExecutionID          string                  `json:"execution_id,omitempty"`
	ScopeID              string                  `json:"scope_id"`
	Purpose              aiengine.Purpose        `json:"purpose,omitempty"`
	AIProviderResourceID string                  `json:"ai_provider_resource_id,omitempty"`
	ModelName            string                  `json:"model_name,omitempty"`
	Profile              aiengine.Profile        `json:"profile,omitempty"`
	Task                 string                  `json:"task,omitempty"`
	Messages             []aiengine.Message      `json:"messages,omitempty"`
	Input                map[string]any          `json:"input,omitempty"`
	Context              aiengine.ContextRequest `json:"context,omitempty"`
	SkillResourceID      string                  `json:"skill_resource_id,omitempty"`
	SkillVersionID       string                  `json:"skill_version_id,omitempty"`
	AgentProfileID       string                  `json:"agent_profile_id,omitempty"`
	WorkflowID           string                  `json:"workflow_id,omitempty"`
	Requirements         aiengine.Requirements   `json:"requirements,omitempty"`
	MaxIterations        int                     `json:"max_iterations,omitempty"`
	MaxToolCalls         int                     `json:"max_tool_calls,omitempty"`
	MaxTokens            int64                   `json:"max_tokens,omitempty"`
	MaxOutputBytes       int                     `json:"max_output_bytes,omitempty"`
	TimeoutSeconds       int                     `json:"timeout_seconds,omitempty"`
	Stream               bool                    `json:"stream,omitempty"`
}

func registerAIEngineRoutes(router chi.Router, engine aiengine.Engine, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	if engine == nil {
		return
	}
	guard := func(permission authorization.Permission) func(http.Handler) http.Handler {
		if requirePermission == nil {
			return func(next http.Handler) http.Handler { return next }
		}
		return requirePermission(permission)
	}
	h := aiEngineHandler{engine: engine}
	router.With(guard(authorization.DiagnosisStart)).Post("/ai-executions", h.execute)
	router.With(guard(authorization.DiagnosisStart)).Post("/ai-executions/{executionID}/cancel", h.cancel)
}

func registerAIEngineEventRoutes(router chi.Router, store aiengine.EventStore, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	if store == nil {
		return
	}
	guard := func(next http.Handler) http.Handler { return next }
	if requirePermission != nil {
		guard = requirePermission(authorization.DiagnosisRead)
	}
	h := aiEngineEventHandler{store: store}
	router.With(guard).Get("/ai-executions/{executionID}/events", h.events)
}

func registerAIEngineToolRoutes(router chi.Router, store aiengine.ToolCallStore, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	if store == nil {
		return
	}
	guard := func(next http.Handler) http.Handler { return next }
	if requirePermission != nil {
		guard = requirePermission(authorization.DiagnosisRead)
	}
	h := aiEngineToolHandler{store: store}
	router.With(guard).Get("/ai-executions/{executionID}/tool-calls", h.list)
}

type aiEngineToolHandler struct{ store aiengine.ToolCallStore }

func (h aiEngineToolHandler) list(w http.ResponseWriter, r *http.Request) {
	executionID := strings.TrimSpace(chi.URLParam(r, "executionID"))
	if executionID == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "executionID is required")
		return
	}
	items, err := h.store.ListToolCalls(r.Context(), executionID, 200)
	if err != nil {
		writeAIEngineError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

type aiEngineEventHandler struct{ store aiengine.EventStore }

func (h aiEngineEventHandler) events(w http.ResponseWriter, r *http.Request) {
	after, err := eventCursor(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	executionID := strings.TrimSpace(chi.URLParam(r, "executionID"))
	if executionID == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "executionID is required")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// ResponseWriter is wrapped by the HTTP observability middleware. Use the
	// response controller so Flush can unwrap middleware writers that expose
	// flushing through FlushError rather than implementing http.Flusher directly.
	controller := http.NewResponseController(w)
	for {
		events, listErr := h.store.ListEvents(r.Context(), executionID, after, 200)
		if listErr != nil {
			return
		}
		for _, event := range events {
			payload, marshalErr := json.Marshal(event.Payload)
			if marshalErr != nil {
				return
			}
			if _, writeErr := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.Sequence, event.Type, payload); writeErr != nil {
				return
			}
			after = event.Sequence
		}
		if err := controller.Flush(); err != nil {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(300 * time.Millisecond):
		}
	}
}

func (h aiEngineHandler) execute(w http.ResponseWriter, r *http.Request) {
	var body executeAIEngineRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.ScopeID) == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "scope_id is required")
		return
	}
	user := currentUser(r)
	request := aiengine.Request{
		ExecutionID: body.ExecutionID, ActorID: user.ID, ScopeID: body.ScopeID,
		Purpose: body.Purpose, AIProviderResourceID: body.AIProviderResourceID, ModelName: body.ModelName, Profile: body.Profile, Task: body.Task,
		Messages: body.Messages, Input: body.Input, Context: body.Context,
		SkillResourceID: body.SkillResourceID, SkillVersionID: body.SkillVersionID,
		AgentProfileID: body.AgentProfileID, WorkflowID: body.WorkflowID,
		Requirements: body.Requirements, Stream: body.Stream,
		Budget: aiengine.Budget{MaxIterations: body.MaxIterations, MaxToolCalls: body.MaxToolCalls, MaxTokens: body.MaxTokens, MaxOutputBytes: body.MaxOutputBytes, Timeout: time.Duration(body.TimeoutSeconds) * time.Second},
	}
	result, err := h.engine.Execute(r.Context(), request)
	if err != nil {
		writeAIEngineError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h aiEngineHandler) cancel(w http.ResponseWriter, r *http.Request) {
	executionID := strings.TrimSpace(chi.URLParam(r, "executionID"))
	if executionID == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "executionID is required")
		return
	}
	if err := h.engine.Cancel(r.Context(), executionID); err != nil {
		if strings.Contains(err.Error(), "not running") {
			writeError(w, r, http.StatusNotFound, "not_found", "AI execution is not running")
			return
		}
		writeAIEngineError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeAIEngineError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}
	if errorsIsContext(err) {
		writeError(w, r, http.StatusGatewayTimeout, "timeout", err.Error())
		return
	}
	if strings.Contains(err.Error(), "request is invalid") || strings.Contains(err.Error(), "required") {
		writeError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeError(w, r, http.StatusBadGateway, "ai_runtime_error", err.Error())
}

func errorsIsContext(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}
