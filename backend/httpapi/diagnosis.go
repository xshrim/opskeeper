package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"opskeeper/backend/audit"
	"opskeeper/backend/authorization"
	"opskeeper/backend/diagnosis"
)

type diagnosisService interface {
	Start(context.Context, diagnosis.StartInput) (diagnosis.Session, error)
	Get(context.Context, string) (diagnosis.Snapshot, error)
	List(context.Context, string, int) ([]diagnosis.Session, error)
	AddTarget(context.Context, string, string) (diagnosis.Target, error)
	Ask(context.Context, string, string) (diagnosis.Message, error)
	EventsAfter(context.Context, string, int64, int) ([]diagnosis.Event, error)
}

type diagnosisCanceller interface {
	Cancel(context.Context, string) error
}

type diagnosisDeleter interface {
	Delete(context.Context, string) error
}

type diagnosisHandler struct {
	service diagnosisService
	auditor audit.Logger
}

type startDiagnosisRequest struct {
	ScopeID            string   `json:"scope_id"`
	Title              string   `json:"title"`
	Question           string   `json:"question"`
	TargetResourceIDs  []string `json:"target_resource_ids"`
	ProviderResourceID string   `json:"ai_provider_resource_id,omitempty"`
	ModelName          string   `json:"model_name,omitempty"`
}

type addDiagnosisTargetRequest struct {
	ResourceID string `json:"resource_id"`
}
type askDiagnosisRequest struct {
	Content string `json:"content"`
}

func registerDiagnosisRoutes(router chi.Router, service diagnosisService, auditor audit.Logger, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	if service == nil {
		return
	}
	h := diagnosisHandler{service: service, auditor: auditor}
	guard := func(permission authorization.Permission) func(http.Handler) http.Handler {
		if requirePermission == nil {
			return func(next http.Handler) http.Handler { return next }
		}
		return requirePermission(permission)
	}
	router.With(guard(authorization.DiagnosisStart)).Post("/diagnosis-sessions", h.start)
	router.With(guard(authorization.DiagnosisRead)).Get("/diagnosis-sessions", h.list)
	router.Route("/diagnosis-sessions/{sessionID}", func(router chi.Router) {
		router.With(guard(authorization.DiagnosisRead)).Get("/", h.get)
		router.With(guard(authorization.DiagnosisStart)).Post("/targets", h.addTarget)
		router.With(guard(authorization.DiagnosisStart)).Post("/messages", h.ask)
		router.With(guard(authorization.DiagnosisStart)).Post("/cancel", h.cancel)
		router.With(guard(authorization.DiagnosisStart)).Delete("/", h.delete)
		router.With(guard(authorization.DiagnosisRead)).Get("/events", h.events)
	})
}

func (h diagnosisHandler) cancel(w http.ResponseWriter, r *http.Request) {
	canceller, ok := h.service.(diagnosisCanceller)
	if !ok {
		writeError(w, r, http.StatusNotImplemented, "cancel_unavailable", "Diagnosis cancellation is unavailable")
		return
	}
	if err := canceller.Cancel(r.Context(), chi.URLParam(r, "sessionID")); err != nil {
		writeDiagnosisError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h diagnosisHandler) delete(w http.ResponseWriter, r *http.Request) {
	deleter, ok := h.service.(diagnosisDeleter)
	if !ok {
		writeError(w, r, http.StatusNotImplemented, "delete_unavailable", "Diagnosis deletion is unavailable")
		return
	}
	if err := deleter.Delete(r.Context(), chi.URLParam(r, "sessionID")); err != nil {
		writeDiagnosisError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h diagnosisHandler) start(w http.ResponseWriter, r *http.Request) {
	var body startDiagnosisRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.service.Start(r.Context(), diagnosis.StartInput{ScopeID: body.ScopeID, ActorUserID: currentUser(r).ID, Title: body.Title, Question: body.Question, TargetResourceIDs: body.TargetResourceIDs, ProviderResourceID: body.ProviderResourceID, ModelName: body.ModelName})
	if err != nil {
		writeDiagnosisError(w, r, err)
		return
	}
	h.record(r, "diagnosis.start", "diagnosis_session", item.ID, item.ScopeID)
	w.Header().Set("Location", r.URL.Path+"/"+item.ID)
	writeJSON(w, http.StatusCreated, item)
}

func (h diagnosisHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, err := positiveLimit(r.URL.Query().Get("limit"), 50, 100)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	items, err := h.service.List(r.Context(), r.URL.Query().Get("scope_id"), limit)
	if err != nil {
		writeDiagnosisError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h diagnosisHandler) get(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.Get(r.Context(), chi.URLParam(r, "sessionID"))
	if err != nil {
		writeDiagnosisError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h diagnosisHandler) addTarget(w http.ResponseWriter, r *http.Request) {
	var body addDiagnosisTargetRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.service.AddTarget(r.Context(), chi.URLParam(r, "sessionID"), body.ResourceID)
	if err != nil {
		writeDiagnosisError(w, r, err)
		return
	}
	h.record(r, "diagnosis.target.add", "diagnosis_session", chi.URLParam(r, "sessionID"), "")
	writeJSON(w, http.StatusCreated, item)
}

func (h diagnosisHandler) ask(w http.ResponseWriter, r *http.Request) {
	var body askDiagnosisRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.service.Ask(r.Context(), chi.URLParam(r, "sessionID"), body.Content)
	if err != nil {
		writeDiagnosisError(w, r, err)
		return
	}
	h.record(r, "diagnosis.message.create", "diagnosis_session", chi.URLParam(r, "sessionID"), "")
	writeJSON(w, http.StatusCreated, item)
}

func (h diagnosisHandler) events(w http.ResponseWriter, r *http.Request) {
	after, err := eventCursor(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	// An authorization-aware service call validates the session before the
	// stream headers are committed, allowing normal JSON errors on rejection.
	if _, err := h.service.EventsAfter(r.Context(), chi.URLParam(r, "sessionID"), after, 1); err != nil {
		writeDiagnosisError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	for {
		events, err := h.service.EventsAfter(r.Context(), chi.URLParam(r, "sessionID"), after, 200)
		if err != nil {
			return
		}
		for _, event := range events {
			if err := writeSSEEvent(w, controller, event); err != nil {
				return
			}
			after = event.ID
			// A diagnosis execution may emit execution.completed before its
			// report is persisted. Keep the stream open through that
			// post-processing window, then close after the durable report or
			// diagnosis terminal marker so clients do not leak an EventSource.
			if terminalDiagnosisEvent(event) {
				return
			}
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(300 * time.Millisecond):
		}
	}
}

func terminalDiagnosisEvent(event diagnosis.Event) bool {
	switch event.Type {
	case "report.ready", "diagnosis.failed", "diagnosis.cancelled":
		return true
	default:
		return false
	}
}

func writeSSEEvent(w http.ResponseWriter, controller *http.ResponseController, event diagnosis.Event) error {
	if err := controller.SetWriteDeadline(time.Now().Add(20 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.ID, event.Type, event.Payload); err != nil {
		return err
	}
	return controller.Flush()
}

func eventCursor(r *http.Request) (int64, error) {
	value := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	if value == "" {
		value = strings.TrimSpace(r.URL.Query().Get("after"))
	}
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, errors.New("Last-Event-ID must be a non-negative integer")
	}
	return parsed, nil
}

func positiveLimit(raw string, fallback, maximum int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > maximum {
		return 0, fmt.Errorf("limit must be between 1 and %d", maximum)
	}
	return value, nil
}

func writeDiagnosisError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, diagnosis.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "Diagnosis session not found")
	case errors.Is(err, diagnosis.ErrConflict):
		writeError(w, r, http.StatusConflict, "conflict", "Diagnosis session is not available for this operation")
	case errors.Is(err, authorization.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "forbidden", "You do not have permission for this diagnosis")
	default:
		if diagnosis.IsInvalid(err) {
			writeError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}

func (h diagnosisHandler) record(r *http.Request, action, targetType, targetID, scopeID string) {
	if h.auditor == nil {
		return
	}
	_ = h.auditor.Record(r.Context(), audit.Event{ActorUserID: currentUser(r).ID, Action: action, TargetType: targetType, TargetID: targetID, ScopeID: scopeID, RequestID: middleware.GetReqID(r.Context()), ClientIP: requestClientIP(r), Details: map[string]any{}})
}
