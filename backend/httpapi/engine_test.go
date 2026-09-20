package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	enginepkg "opskeeper/backend/engine"
	"opskeeper/backend/identity"
)

type fakeEngine struct{ request enginepkg.Request }

func (f *fakeEngine) Name() string { return "fake" }
func (f *fakeEngine) Execute(_ context.Context, request enginepkg.Request) (enginepkg.Result, error) {
	f.request = request
	return enginepkg.Result{ExecutionID: "exec-1", Status: enginepkg.StatusSucceeded, Output: "ok"}, nil
}
func (f *fakeEngine) Stream(context.Context, enginepkg.Request) (<-chan enginepkg.Event, error) {
	return nil, nil
}
func (f *fakeEngine) Cancel(context.Context, string) error { return nil }

func TestEngineHandlerUsesAuthenticatedActorAndStructuredInput(t *testing.T) {
	fake := &fakeEngine{}
	handler := engineHandler{engine: fake}
	body := `{"scope_id":"scope-1","profile":"diagnosis","input":{"question":"status"}}`
	request := httptest.NewRequest(http.MethodPost, "/ai-executions", strings.NewReader(body))
	request = request.WithContext(context.WithValue(request.Context(), authenticatedUserContextKey{}, identity.User{ID: "user-1"}))
	response := httptest.NewRecorder()
	handler.execute(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if fake.request.ActorID != "user-1" || fake.request.ScopeID != "scope-1" || fake.request.Input["question"] != "status" {
		t.Fatalf("request=%+v", fake.request)
	}
	var result enginepkg.Result
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.Output != "ok" {
		t.Fatalf("response=%s err=%v", response.Body.String(), err)
	}
}

func TestEngineHandlerPassesSkillAndPersonaSelection(t *testing.T) {
	fake := &fakeEngine{}
	handler := engineHandler{engine: fake}
	request := httptest.NewRequest(http.MethodPost, "/ai-executions", strings.NewReader(`{"scope_id":"scope-1","purpose":"diagnosis","skill_id":"skill-1","skill_version_id":"version-2","persona_id":"agent-1","task":"inspect"}`))
	request = request.WithContext(context.WithValue(request.Context(), authenticatedUserContextKey{}, identity.User{ID: "user-1"}))
	response := httptest.NewRecorder()
	handler.execute(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if fake.request.Purpose != enginepkg.PurposeDiagnosis || fake.request.SkillID != "skill-1" || fake.request.SkillVersionID != "version-2" || fake.request.PersonaID != "agent-1" {
		t.Fatalf("request selection=%+v", fake.request)
	}
}

func TestTerminalAIEventClosesStreamAfterFinalLifecycleEvent(t *testing.T) {
	for _, eventType := range []string{"execution.completed", "execution.failed", "execution.cancelled"} {
		if !terminalAIEvent(enginepkg.Event{Type: eventType}) {
			t.Errorf("terminalAIEvent(%q) = false", eventType)
		}
	}
	if terminalAIEvent(enginepkg.Event{Type: "tool.completed"}) {
		t.Fatal("tool.completed must not close the execution stream")
	}
}

func TestEngineErrorResponseRedactsUpstreamCredentials(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		code string
	}{
		{name: "runtime", err: errors.New("upstream returned authorization: Bearer super-secret-token"), code: "ai_runtime_error"},
		{name: "json", err: errors.New(`upstream payload: {"api_key":"super-secret-json-key"}`), code: "ai_runtime_error"},
		{name: "invalid", err: errors.New("request is invalid: api_key=super-secret-key"), code: "invalid_request"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/ai-executions", nil)
			response := httptest.NewRecorder()
			writeEngineError(response, request, test.err)
			if response.Code != http.StatusBadGateway && test.code == "ai_runtime_error" {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if test.code == "invalid_request" && response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			body := response.Body.String()
			if strings.Contains(body, "super-secret") {
				t.Fatalf("credential leaked in response: %s", body)
			}
			if !strings.Contains(body, test.code) {
				t.Fatalf("error code missing from response: %s", body)
			}
		})
	}
}

func TestEngineErrorResponseUsesTerminalContextMessages(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
		text   string
	}{
		{name: "timeout", err: context.DeadlineExceeded, status: http.StatusGatewayTimeout, text: "AI execution timed out"},
		{name: "cancelled", err: context.Canceled, status: http.StatusRequestTimeout, text: "AI execution was cancelled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/ai-executions", nil)
			response := httptest.NewRecorder()
			writeEngineError(response, request, test.err)
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.text) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}
