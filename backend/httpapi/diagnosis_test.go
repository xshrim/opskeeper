package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"opskeeper/backend/diagnosis"
	"opskeeper/backend/identity"
)

var testDiagnosisHTTPUser = identity.User{ID: "user-1"}

func TestDiagnosisStartPassesActorAndPayload(t *testing.T) {
	service := &stubDiagnosisService{}
	handler := diagnosisHandler{service: service}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/diagnosis-sessions", strings.NewReader(`{"scope_id":"scope-1","question":"why","target_resource_ids":["resource-1"]}`))
	request = request.WithContext(context.WithValue(request.Context(), authenticatedUserContextKey{}, testDiagnosisHTTPUser))
	response := httptest.NewRecorder()

	handler.start(response, request)

	if response.Code != http.StatusCreated || service.start.ActorUserID != testDiagnosisHTTPUser.ID || service.start.ScopeID != "scope-1" || len(service.start.TargetResourceIDs) != 1 {
		t.Fatalf("status=%d input=%#v body=%s", response.Code, service.start, response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload["id"] != "session-1" || payload["scope_id"] != "scope-1" {
		t.Fatalf("diagnosis session JSON payload = %#v, err=%v", payload, err)
	}
}

func TestDiagnosisEventsRejectsInvalidCursor(t *testing.T) {
	handler := diagnosisHandler{service: &stubDiagnosisService{}}
	request := routeRequest(http.MethodGet, "/api/v1/diagnosis-sessions/session-1/events?after=-1", "session-1")
	response := httptest.NewRecorder()

	handler.events(response, request)

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "Last-Event-ID") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestDiagnosisEventsStreamsOnlyEventsAfterLastEventID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	service := &stubDiagnosisService{cancel: cancel, events: []diagnosis.Event{{ID: 4, SessionID: "session-1", Type: "phase.changed", Payload: json.RawMessage(`{"phase":"collecting"}`), CreatedAt: time.Now()}}}
	handler := diagnosisHandler{service: service}
	request := routeRequest(http.MethodGet, "/api/v1/diagnosis-sessions/session-1/events", "session-1").WithContext(ctx)
	request.Header.Set("Last-Event-ID", "3")
	response := httptest.NewRecorder()

	handler.events(response, request)

	if response.Header().Get("Content-Type") != "text/event-stream" || !strings.Contains(response.Body.String(), "id: 4\nevent: phase.changed") {
		t.Fatalf("headers=%v body=%q", response.Header(), response.Body.String())
	}
	if service.after != 3 {
		t.Fatalf("after=%d, want Last-Event-ID cursor 3", service.after)
	}
}

func routeRequest(method, path, sessionID string) *http.Request {
	request := httptest.NewRequest(method, path, nil)
	route := chi.NewRouteContext()
	route.URLParams.Add("sessionID", sessionID)
	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, route))
}

type stubDiagnosisService struct {
	start  diagnosis.StartInput
	events []diagnosis.Event
	after  int64
	cancel context.CancelFunc
}

func (s *stubDiagnosisService) Start(_ context.Context, input diagnosis.StartInput) (diagnosis.Session, error) {
	s.start = input
	return diagnosis.Session{ID: "session-1", ScopeID: input.ScopeID, Status: diagnosis.StatusQueued}, nil
}
func (*stubDiagnosisService) Get(context.Context, string) (diagnosis.Snapshot, error) {
	return diagnosis.Snapshot{}, nil
}
func (*stubDiagnosisService) List(context.Context, string, int) ([]diagnosis.Session, error) {
	return nil, nil
}
func (*stubDiagnosisService) AddTarget(context.Context, string, string) (diagnosis.Target, error) {
	return diagnosis.Target{}, nil
}
func (*stubDiagnosisService) Ask(context.Context, string, string) (diagnosis.Message, error) {
	return diagnosis.Message{}, nil
}
func (s *stubDiagnosisService) EventsAfter(_ context.Context, _ string, after int64, limit int) ([]diagnosis.Event, error) {
	s.after = after
	if limit == 200 && s.cancel != nil {
		s.cancel()
		return s.events, nil
	}
	return s.events, nil
}
