package skill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool/functiontool"
	"opskeeper/backend/aiengine"
	"opskeeper/backend/authorization"
	"opskeeper/backend/connector"
	"opskeeper/backend/llm"
	"opskeeper/backend/resource"
)

func TestAIEngineAdapterPreservesProviderSelection(t *testing.T) {
	request := aiengine.Request{AIProviderResourceID: "provider-1", ModelName: "model-a"}
	if request.AIProviderResourceID != "provider-1" || request.ModelName != "model-a" {
		t.Fatalf("request did not preserve AIProvider selection")
	}
}

func TestRunnerUsesADKToolLoopAndPersistsPinnedExecution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Messages []struct {
				Role string `json:"role"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		writer.Header().Set("Content-Type", "application/json")
		if len(body.Messages) > 0 && body.Messages[len(body.Messages)-1].Role == "tool" {
			fmt.Fprint(writer, `{"model":"mock","choices":[{"finish_reason":"stop","message":{"content":"{\"status\":\"healthy\"}"}}],"usage":{"prompt_tokens":9,"completion_tokens":4,"total_tokens":13}}`)
			return
		}
		fmt.Fprint(writer, `{"model":"mock","choices":[{"finish_reason":"tool_calls","message":{"content":"","tool_calls":[{"id":"call-1","type":"function","function":{"name":"connector_kubernetes_read","arguments":"{\"target_resource_id\":\"target-1\",\"resource\":\"pods\"}"}}]}}],"usage":{"prompt_tokens":7,"completion_tokens":2,"total_tokens":9}}`)
	}))
	defer server.Close()

	resources := fakeResourceReader{items: map[string]resource.Resource{
		"provider-1": {ID: "provider-1", ScopeID: "scope-1", Kind: llm.AIProviderKind, Name: "mock provider", Status: resource.StatusActive, Config: map[string]any{"provider_type": "openai_compatible", "base_url": server.URL, "enabled": true, "default_model": "mock", "models": []any{map[string]any{"name": "mock", "context_window_tokens": 8192.0, "temperature": 0.7, "capabilities": []any{"text", "tool_calling", "stream"}, "enabled": true}}}},
		"skill-1":    {ID: "skill-1", ScopeID: "scope-1", Kind: Kind, Name: "health", Status: resource.StatusActive},
		"target-1":   {ID: "target-1", ScopeID: "scope-1", Kind: "Application", Name: "api", Status: resource.StatusActive},
	}}
	version := Version{ID: "version-1", SkillResourceID: "skill-1", Version: 1, Status: "published", Manifest: Manifest{Name: "health", Instruction: "Inspect the target and return JSON.", TargetKinds: []string{"Application"}}, InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object","required":["status"]}`), Tools: []ToolSpec{{Name: "connector_kubernetes_read", Description: "read Kubernetes objects"}}, RiskLevel: "read_only"}
	store := &memoryStore{version: version}
	modelService := llm.NewService(fakeLLMStore{}, resources, nil)
	skillService := NewService(store, resources)
	connectorService := &fakeConnector{}
	runtime := NewRunner(skillService, modelService, connectorService, store)

	result, err := runtime.Run(context.Background(), RunInput{ActorID: "actor-1", ScopeID: "scope-1", TargetResourceID: "target-1", SkillResourceID: "skill-1", SkillVersionID: "version-1", AIProviderResourceID: "provider-1", Input: map[string]any{"question": "health"}, MaxIterations: 2, MaxToolCalls: 2, MaxTokens: 100, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Execution.Status != "succeeded" || result.Execution.SkillVersionID != "version-1" || result.Execution.ProviderResourceID != "provider-1" || result.Execution.ModelName != "mock" {
		t.Fatalf("execution = %#v", result.Execution)
	}
	if result.Execution.TotalTokens != 22 || result.Execution.ToolCallCount != 1 {
		t.Fatalf("usage = %#v", result.Execution)
	}
	if result.Output != `{"status":"healthy"}` {
		t.Errorf("output = %q", result.Output)
	}
	if connectorService.reads != 1 {
		t.Errorf("connector reads = %d", connectorService.reads)
	}
	if len(store.toolCalls) != 1 || store.toolCalls[0].Status != "succeeded" {
		t.Fatalf("tool calls = %#v", store.toolCalls)
	}
}

func TestCountsOnlyCompletedModelResponsesAsIterations(t *testing.T) {
	for _, test := range []struct {
		name        string
		partial     bool
		role        string
		wantCounted bool
	}{
		{name: "completed model turn", role: "model", wantCounted: true},
		{name: "partial model chunk", partial: true, role: "model"},
		{name: "tool result", role: "user"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := countsModelIteration(test.partial, test.role); got != test.wantCounted {
				t.Fatalf("countsModelIteration() = %v, want %v", got, test.wantCounted)
			}
		})
	}
}

func TestPolicyRejectsUndeclaredInvalidOverBudgetAndUnauthorizedTools(t *testing.T) {
	toolItem, err := functiontool.New(functiontool.Config{Name: "connector_kubernetes_read", Description: "read"}, func(agent.Context, k8sArgs) (map[string]any, error) { return map[string]any{}, nil })
	if err != nil {
		t.Fatal(err)
	}
	undeclared, err := functiontool.New(functiontool.Config{Name: "other_tool", Description: "other"}, func(agent.Context, map[string]any) (map[string]any, error) { return map[string]any{}, nil })
	if err != nil {
		t.Fatal(err)
	}
	resources := fakeResourceReader{items: map[string]resource.Resource{"target-1": {ID: "target-1", ScopeID: "scope-1"}, "target-2": {ID: "target-2", ScopeID: "scope-1"}}}
	auditStore := &memoryStore{}
	policy := &policy{runner: &Runner{Skills: NewService(auditStore, resources), Executions: auditStore}, runCtx: context.Background(), executionID: "execution-1", version: Version{Tools: []ToolSpec{{Name: "connector_kubernetes_read", InputSchema: json.RawMessage(`{"type":"object","required":["resource"],"properties":{"resource":{"type":"string"},"target_resource_id":{"type":"string"}},"additionalProperties":false}`)}}}, targetResourceID: "target-1", resourceFilter: authorization.ResourceFilter{ResourceIDs: []string{"target-1"}}, calls: 1}
	if _, err := policy.beforeTool(nil, undeclared, map[string]any{}); err == nil {
		t.Fatal("undeclared tool was allowed")
	}
	if _, err := policy.beforeTool(nil, toolItem, map[string]any{"target_resource_id": "target-1"}); err == nil {
		t.Fatal("invalid tool arguments were allowed")
	}
	if _, err := policy.beforeTool(nil, toolItem, map[string]any{"target_resource_id": "target-2", "resource": "pods"}); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("unauthorized target error = %v", err)
	}
	policy.usedCalls.Store(0)
	if _, err := policy.beforeTool(nil, toolItem, map[string]any{"target_resource_id": "target-1", "resource": "pods"}); err != nil {
		t.Fatalf("first allowed call error = %v", err)
	}
	if _, err := policy.beforeTool(nil, toolItem, map[string]any{"target_resource_id": "target-1", "resource": "pods"}); !errors.Is(err, ErrBudget) {
		t.Fatalf("budget error = %v", err)
	}
	if len(auditStore.toolCalls) != 4 {
		t.Fatalf("rejected tool calls = %#v", auditStore.toolCalls)
	}
	for _, call := range auditStore.toolCalls {
		if call.Status != "rejected" || call.ErrorCode == "" {
			t.Fatalf("rejected call was not persisted: %#v", call)
		}
	}
}

func TestPolicyBuildsOnlyDeclaredMiddlewareInspectionTools(t *testing.T) {
	policy := &policy{
		runner:  &Runner{Connector: &fakeConnector{}},
		version: Version{Tools: []ToolSpec{{Name: "connector_postgresql_inspect"}, {Name: "connector_redis_inspect"}, {Name: "connector_kafka_inspect"}}},
	}
	tools, err := policy.tools(context.Background())
	if err != nil {
		t.Fatalf("tools() error = %v", err)
	}
	if len(tools) != 3 || tools[0].Name() != "connector_postgresql_inspect" || tools[1].Name() != "connector_redis_inspect" || tools[2].Name() != "connector_kafka_inspect" {
		t.Fatalf("tools() = %#v", tools)
	}
}

func TestServiceRequiresAccessToTheSkillResource(t *testing.T) {
	resources := fakeResourceReader{items: map[string]resource.Resource{
		"skill-1": {ID: "skill-1", ScopeID: "scope-1", Kind: Kind, Name: "allowed skill", Status: resource.StatusActive},
		"skill-2": {ID: "skill-2", ScopeID: "scope-1", Kind: Kind, Name: "other skill", Status: resource.StatusActive},
	}}
	store := &memoryStore{version: Version{ID: "version-1", SkillResourceID: "skill-1", Status: "published"}}
	service := NewService(store, resources)

	denied := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{ResourceIDs: []string{"skill-2"}})
	if _, err := service.ListVersions(denied, "skill-1"); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("ListVersions() error = %v, want ErrForbidden", err)
	}
	if _, err := service.Resolve(denied, "scope-1", "skill-1", "version-1"); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("Resolve() error = %v, want ErrForbidden", err)
	}

	allowed := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{ResourceIDs: []string{"skill-1"}})
	if _, err := service.ListVersions(allowed, "skill-1"); err != nil {
		t.Fatalf("ListVersions() allowed error = %v", err)
	}
	if _, err := service.Resolve(allowed, "scope-1", "skill-1", "version-1"); err != nil {
		t.Fatalf("Resolve() allowed error = %v", err)
	}
}

type fakeResourceReader struct{ items map[string]resource.Resource }

func (f fakeResourceReader) Get(_ context.Context, id string) (resource.Resource, error) {
	item, ok := f.items[id]
	if !ok {
		return resource.Resource{}, resource.ErrNotFound
	}
	return item, nil
}

type fakeLLMStore struct{}

func (fakeLLMStore) SetBinding(context.Context, llm.ScopeProviderBinding, string) (llm.ScopeProviderBinding, error) {
	return llm.ScopeProviderBinding{}, errors.New("not implemented")
}
func (fakeLLMStore) RemoveBinding(context.Context, string, llm.Purpose) error {
	return errors.New("not implemented")
}
func (fakeLLMStore) ListBindings(context.Context, string) ([]llm.ScopeProviderBinding, error) {
	return nil, nil
}
func (fakeLLMStore) ResolveBinding(context.Context, string, llm.Purpose) (llm.ScopeProviderBinding, error) {
	return llm.ScopeProviderBinding{}, llm.ErrNotFound
}

type fakeConnector struct{ reads int }

func (f *fakeConnector) ReadKubernetes(_ context.Context, _ string, _ connector.KubernetesQuery) (connector.Evidence, error) {
	f.reads++
	return connector.Evidence{Data: json.RawMessage(`{"items":[{"name":"api-1"}]}`), Summary: map[string]any{"count": 1}}, nil
}
func (*fakeConnector) QueryMetrics(context.Context, string, connector.MetricsQuery) (connector.Evidence, error) {
	return connector.Evidence{}, connector.ErrUnsupported
}
func (*fakeConnector) QueryLogs(context.Context, string, connector.LogsQuery) (connector.Evidence, error) {
	return connector.Evidence{}, connector.ErrUnsupported
}
func (*fakeConnector) QueryTraces(context.Context, string, connector.TracesQuery) (connector.Evidence, error) {
	return connector.Evidence{}, connector.ErrUnsupported
}
func (*fakeConnector) GetAlerts(context.Context, string, connector.AlertsQuery) (connector.Evidence, error) {
	return connector.Evidence{}, connector.ErrUnsupported
}
func (*fakeConnector) InspectPostgreSQL(context.Context, string) (connector.Evidence, error) {
	return connector.Evidence{}, connector.ErrUnsupported
}
func (*fakeConnector) InspectRedis(context.Context, string) (connector.Evidence, error) {
	return connector.Evidence{}, connector.ErrUnsupported
}
func (*fakeConnector) InspectKafka(context.Context, string) (connector.Evidence, error) {
	return connector.Evidence{}, connector.ErrUnsupported
}

type memoryStore struct {
	mu        sync.Mutex
	version   Version
	execution Execution
	toolCalls []ToolCall
}

func (m *memoryStore) CreateVersion(context.Context, CreateVersionInput) (Version, error) {
	return Version{}, errors.New("not implemented")
}
func (m *memoryStore) GetVersion(_ context.Context, id string) (Version, error) {
	if id != m.version.ID {
		return Version{}, ErrNotFound
	}
	return m.version, nil
}
func (m *memoryStore) ListVersions(context.Context, string) ([]Version, error) {
	return []Version{m.version}, nil
}
func (m *memoryStore) PublishVersion(context.Context, string, string) (Version, error) {
	return Version{}, errors.New("not implemented")
}
func (m *memoryStore) DisableVersion(context.Context, string, string) (Version, error) {
	return Version{}, errors.New("not implemented")
}
func (m *memoryStore) SetDefault(context.Context, Default, string) (Default, error) {
	return Default{}, errors.New("not implemented")
}
func (m *memoryStore) ResolveDefault(context.Context, string) (Default, error) {
	return Default{}, ErrNotFound
}
func (m *memoryStore) StartExecution(_ context.Context, input StartExecutionInput) (Execution, error) {
	m.execution = Execution{ID: "execution-1", ScopeID: input.ScopeID, SkillResourceID: input.SkillResourceID, SkillVersionID: input.SkillVersionID, ProviderResourceID: input.ProviderResourceID, ModelName: input.ModelName, Status: "running", InputDigest: input.InputDigest}
	return m.execution, nil
}
func (m *memoryStore) FinishExecution(_ context.Context, _ string, input FinishExecutionInput) (Execution, error) {
	m.execution.Status = input.Status
	m.execution.OutputPreview = input.OutputPreview
	m.execution.PromptTokens = input.PromptTokens
	m.execution.CompletionTokens = input.CompletionTokens
	m.execution.TotalTokens = input.TotalTokens
	m.execution.ToolCallCount = input.ToolCallCount
	m.execution.ErrorCode = input.ErrorCode
	m.execution.ErrorMessage = input.ErrorMessage
	return m.execution, nil
}
func (m *memoryStore) GetExecution(context.Context, string) (Execution, error) {
	return m.execution, nil
}
func (m *memoryStore) ListExecutions(context.Context, string, int) ([]Execution, error) {
	return []Execution{m.execution}, nil
}
func (m *memoryStore) StartToolCall(_ context.Context, executionID string, sequence int, name, targetID, digest string) (ToolCall, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	call := ToolCall{ID: fmt.Sprintf("call-%d", sequence), ExecutionID: executionID, Sequence: sequence, ToolName: name, Status: "running", InputDigest: digest}
	if targetID != "" {
		call.TargetResourceID = &targetID
	}
	m.toolCalls = append(m.toolCalls, call)
	return call, nil
}
func (m *memoryStore) FinishToolCall(_ context.Context, id, status, preview, code, message string) (ToolCall, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.toolCalls {
		if m.toolCalls[index].ID == id {
			m.toolCalls[index].Status = status
			m.toolCalls[index].OutputPreview = preview
			m.toolCalls[index].ErrorCode = code
			m.toolCalls[index].ErrorMessage = message
			return m.toolCalls[index], nil
		}
	}
	return ToolCall{}, ErrNotFound
}
