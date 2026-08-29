package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"opskeeper/backend/aiengine"
	"opskeeper/backend/identity"
)

type fakeAIEngine struct{ request aiengine.Request }

func (f *fakeAIEngine) Name() string { return "fake" }
func (f *fakeAIEngine) Execute(_ context.Context, request aiengine.Request) (aiengine.Result, error) {
	f.request = request
	return aiengine.Result{ExecutionID: "exec-1", Status: aiengine.StatusSucceeded, Output: "ok"}, nil
}
func (f *fakeAIEngine) Stream(context.Context, aiengine.Request) (<-chan aiengine.Event, error) {
	return nil, nil
}
func (f *fakeAIEngine) Cancel(context.Context, string) error { return nil }

func TestAIEngineHandlerUsesAuthenticatedActorAndStructuredInput(t *testing.T) {
	engine := &fakeAIEngine{}
	handler := aiEngineHandler{engine: engine}
	body := `{"scope_id":"scope-1","profile":"diagnosis","input":{"question":"status"}}`
	request := httptest.NewRequest(http.MethodPost, "/ai-executions", strings.NewReader(body))
	request = request.WithContext(context.WithValue(request.Context(), authenticatedUserContextKey{}, identity.User{ID: "user-1"}))
	response := httptest.NewRecorder()
	handler.execute(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if engine.request.ActorID != "user-1" || engine.request.ScopeID != "scope-1" || engine.request.Input["question"] != "status" {
		t.Fatalf("request=%+v", engine.request)
	}
	var result aiengine.Result
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.Output != "ok" {
		t.Fatalf("response=%s err=%v", response.Body.String(), err)
	}
}

func TestAIEngineHandlerPassesSkillAndAgentProfileSelection(t *testing.T) {
	engine := &fakeAIEngine{}
	handler := aiEngineHandler{engine: engine}
	request := httptest.NewRequest(http.MethodPost, "/ai-executions", strings.NewReader(`{"scope_id":"scope-1","purpose":"diagnosis","skill_resource_id":"skill-1","skill_version_id":"version-2","agent_profile_id":"agent-1","task":"inspect"}`))
	request = request.WithContext(context.WithValue(request.Context(), authenticatedUserContextKey{}, identity.User{ID: "user-1"}))
	response := httptest.NewRecorder()
	handler.execute(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if engine.request.Purpose != aiengine.PurposeDiagnosis || engine.request.SkillResourceID != "skill-1" || engine.request.SkillVersionID != "version-2" || engine.request.AgentProfileID != "agent-1" {
		t.Fatalf("request selection=%+v", engine.request)
	}
}

func TestTerminalAIEventClosesStreamAfterFinalLifecycleEvent(t *testing.T) {
	for _, eventType := range []string{"execution.completed", "execution.failed", "execution.cancelled"} {
		if !terminalAIEvent(aiengine.Event{Type: eventType}) {
			t.Errorf("terminalAIEvent(%q) = false", eventType)
		}
	}
	if terminalAIEvent(aiengine.Event{Type: "tool.completed"}) {
		t.Fatal("tool.completed must not close the execution stream")
	}
}
