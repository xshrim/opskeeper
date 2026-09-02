package aiengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

type loopTestModel struct {
	mu       sync.Mutex
	turns    int
	requests []*model.LLMRequest
}

type duplicateToolModel struct {
	mu       sync.Mutex
	turns    int
	requests []*model.LLMRequest
}

type outputCapModel struct {
	mu       sync.Mutex
	requests []*model.LLMRequest
}

func (m *outputCapModel) Name() string { return "output-cap-model" }

func (m *outputCapModel) GenerateContent(_ context.Context, request *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		m.mu.Lock()
		m.requests = append(m.requests, request)
		m.mu.Unlock()
		yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "完成"}}}}, nil)
	}
}

func TestAgentRunnerNeverExceedsProviderOutputCap(t *testing.T) {
	modelClient := &outputCapModel{}
	runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
		return ModelBuildResult{
			Client: modelClient, Capabilities: []string{"text"},
			ContextWindowTokens: 20000, MaxOutputTokens: 8192,
		}, nil
	})
	if _, err := runner.Run(context.Background(), Request{
		ExecutionID: "exec-output-cap", ActorID: "actor-1", ScopeID: "scope-1", Task: "answer",
		Budget: Budget{MaxIterations: 1, MaxTokens: 1000, MaxOutputBytes: 4096},
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	modelClient.mu.Lock()
	defer modelClient.mu.Unlock()
	if len(modelClient.requests) != 1 || modelClient.requests[0].Config == nil {
		t.Fatalf("model request config missing: %+v", modelClient.requests)
	}
	if got := modelClient.requests[0].Config.MaxOutputTokens; got != 8192 {
		t.Fatalf("MaxOutputTokens = %d, want provider cap 8192", got)
	}
}

func (m *duplicateToolModel) Name() string { return "duplicate-tool-model" }

func (m *duplicateToolModel) GenerateContent(_ context.Context, request *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		m.mu.Lock()
		m.turns++
		turn := m.turns
		m.requests = append(m.requests, request)
		m.mu.Unlock()
		if turn == 1 {
			var selected string
			var names []string
			if request.Config != nil {
				for _, group := range request.Config.Tools {
					if group == nil {
						continue
					}
					for _, declaration := range group.FunctionDeclarations {
						if declaration != nil {
							if declaration.Name == "read_observation" {
								continue
							}
							names = append(names, declaration.Name)
							if strings.Contains(declaration.Name, "resource_2") {
								selected = declaration.Name
							}
						}
					}
				}
			}
			if len(names) != 2 || names[0] == names[1] || selected == "" {
				yield(nil, fmt.Errorf("duplicate tool declarations: %v", names))
				return
			}
			yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{ID: "duplicate-call", Name: selected, Args: map[string]any{}}}}}}, nil)
			return
		}
		yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "已从指定资源获取结果。"}}}}, nil)
	}
}

func TestAgentRunnerNamespacesDuplicateResourceTools(t *testing.T) {
	modelClient := &duplicateToolModel{}
	registry := NewToolRegistry()
	calledResource := ""
	for _, resourceID := range []string{"resource-1", "resource-2"} {
		resourceID := resourceID
		if err := registry.Register(resourceID, ToolFunc{Def: ToolDefinition{Name: "query", Source: "mcp", ResourceID: resourceID}, Fn: func(context.Context, map[string]any) (ToolResult, error) {
			calledResource = resourceID
			return ToolResult{Output: map[string]any{"resource": resourceID}}, nil
		}}); err != nil {
			t.Fatal(err)
		}
	}
	runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
		return ModelBuildResult{Client: modelClient, Capabilities: []string{"text", "tool_calling"}}, nil
	})
	result, err := runner.Run(context.Background(), Request{
		ExecutionID: "exec-duplicate-tools", ActorID: "actor-1", ScopeID: "scope-1", Task: "query",
		Budget: Budget{MaxIterations: 3, MaxToolCalls: 3, MaxTokens: 1000, MaxOutputBytes: 4096},
		ResolvedContext: &ResolvedContext{Tools: []ToolDefinition{
			{Name: "query", Source: "mcp", ResourceID: "resource-1"},
			{Name: "query", Source: "mcp", ResourceID: "resource-2"},
		}},
		ToolGateway: NewPolicyGateway(registry, nil, 0, 4096, 1),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Output != "已从指定资源获取结果。" || calledResource != "resource-2" {
		t.Fatalf("result=%+v calledResource=%q", result, calledResource)
	}
}

func (m *loopTestModel) Name() string { return "loop-test-model" }

func (m *loopTestModel) GenerateContent(_ context.Context, request *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		m.mu.Lock()
		m.turns++
		turn := m.turns
		m.requests = append(m.requests, request)
		m.mu.Unlock()
		if turn == 1 {
			if stream {
				yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "先检查目标资源。"}}}, Partial: true}, nil)
			}
			yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{
				{Text: "先检查目标资源。"},
				{FunctionCall: &genai.FunctionCall{ID: "call-1", Name: "lookup", Args: map[string]any{"name": "container"}}},
			}}}, nil)
			return
		}
		yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "已根据工具观察完成诊断。"}}}}, nil)
	}
}

func TestAgentRunnerStreamsReActTurnsAndObservations(t *testing.T) {
	modelClient := &loopTestModel{}
	registry := NewToolRegistry()
	if err := registry.Register("resource-1", ToolFunc{Def: ToolDefinition{Name: "lookup", Source: "test", ResourceID: "resource-1"}, Fn: func(context.Context, map[string]any) (ToolResult, error) {
		return ToolResult{Output: map[string]any{"status": "running", "password": "hidden"}, Untrusted: true}, nil
	}}); err != nil {
		t.Fatal(err)
	}
	gateway := NewPolicyGateway(registry, nil, 0, 4096, 1)
	var events []Event
	runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
		return ModelBuildResult{Client: modelClient, Capabilities: []string{"text", "tool_calling", "stream"}}, nil
	})
	result, err := runner.RunStream(context.Background(), Request{
		ExecutionID:     "exec-loop",
		ActorID:         "actor-1",
		ScopeID:         "scope-1",
		Task:            "diagnose container",
		Stream:          true,
		Budget:          Budget{MaxIterations: 4, MaxToolCalls: 4, MaxTokens: 1000, MaxOutputBytes: 4096},
		ResolvedContext: &ResolvedContext{Tools: []ToolDefinition{{Name: "lookup", Source: "test", ResourceID: "resource-1"}}},
		ToolGateway:     gateway,
	}, func(event Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("RunStream() error = %v", err)
	}
	if result.Output != "已根据工具观察完成诊断。" || result.ToolCallCount != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	var types []string
	for _, event := range events {
		types = append(types, event.Type)
	}
	joined := strings.Join(types, ",")
	wantOrder := []string{"model.started", "assistant.delta", "assistant.progress", "tool.requested", "tool.started", "tool.completed", "model.resumed", "model.started", "assistant.completed"}
	last := -1
	for _, wanted := range wantOrder {
		found := -1
		for index := last + 1; index < len(types); index++ {
			if types[index] == wanted {
				found = index
				break
			}
		}
		if found < 0 {
			t.Fatalf("event order missing %q in %s", wanted, joined)
		}
		last = found
	}
	for _, event := range events {
		if event.Type == "tool.completed" {
			if event.Payload["iteration"] != 1 || event.Payload["call_id"] != "call-1" {
				t.Fatalf("tool event lost loop metadata: %+v", event)
			}
			if fmt.Sprint(event.Payload["action_index"]) != "1" || fmt.Sprint(event.Payload["action_count"]) != "1" {
				t.Fatalf("tool event lost server action accounting: %+v", event)
			}
			if elapsed, ok := event.Payload["elapsed_ms"].(int64); !ok || elapsed < 0 {
				t.Fatalf("tool event has invalid server elapsed time: %+v", event)
			}
		}
		if event.Type == "assistant.progress" && event.Payload["final"] != false {
			t.Fatalf("progress event should be marked intermediate: %+v", event)
		}
	}
	modelClient.mu.Lock()
	defer modelClient.mu.Unlock()
	if len(modelClient.requests) != 2 {
		t.Fatalf("model calls = %d, want 2", len(modelClient.requests))
	}
	seenObservation := false
	seenSecret := false
	for _, content := range modelClient.requests[1].Contents {
		for _, part := range content.Parts {
			if part.FunctionResponse != nil {
				seenObservation = true
				encoded, _ := json.Marshal(part.FunctionResponse.Response)
				seenSecret = seenSecret || strings.Contains(string(encoded), "hidden")
			}
		}
	}
	if !seenObservation {
		t.Fatal("second model turn did not receive the tool observation")
	}
	if seenSecret {
		t.Fatal("raw tool secret was passed to the second model turn")
	}
}

type parallelBatchModel struct {
	mu       sync.Mutex
	turns    int
	requests []*model.LLMRequest
}

func (m *parallelBatchModel) Name() string { return "parallel-batch-model" }

func (m *parallelBatchModel) GenerateContent(_ context.Context, request *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		m.mu.Lock()
		m.turns++
		turn := m.turns
		m.requests = append(m.requests, request)
		m.mu.Unlock()
		if turn == 1 {
			yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{
				{Text: "并行确认两个独立信息源。"},
				{FunctionCall: &genai.FunctionCall{ID: "parallel-1", Name: "lookup_one", Args: map[string]any{}}},
				{FunctionCall: &genai.FunctionCall{ID: "parallel-2", Name: "lookup_two", Args: map[string]any{}}},
			}}}, nil)
			return
		}
		responses := 0
		for _, content := range request.Contents {
			for _, part := range content.Parts {
				if part != nil && part.FunctionResponse != nil {
					responses++
				}
			}
		}
		if responses != 2 {
			yield(nil, fmt.Errorf("second turn received %d observations, want 2", responses))
			return
		}
		yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "已汇总并行观察。"}}}}, nil)
	}
}

type parallelProbe struct {
	mu          sync.Mutex
	arrived     int
	running     int
	max         int
	release     chan struct{}
	releaseOnce sync.Once
}

func (p *parallelProbe) waitForBatch(ctx context.Context) {
	p.mu.Lock()
	p.arrived++
	if p.arrived == 2 {
		p.releaseOnce.Do(func() { close(p.release) })
	}
	p.mu.Unlock()
	select {
	case <-p.release:
	case <-ctx.Done():
	}
}

func (p *parallelProbe) enter() {
	p.mu.Lock()
	p.running++
	if p.running > p.max {
		p.max = p.running
	}
	p.mu.Unlock()
}

func (p *parallelProbe) leave() {
	p.mu.Lock()
	p.running--
	p.mu.Unlock()
}

func TestAgentRunnerExecutesIndependentToolCallsInParallel(t *testing.T) {
	probe := &parallelProbe{release: make(chan struct{})}
	registry := NewToolRegistry()
	for _, name := range []string{"lookup_one", "lookup_two"} {
		name := name
		if err := registry.Register("resource-1", ToolFunc{Def: ToolDefinition{Name: name, Source: "test", ResourceID: "resource-1"}, Fn: func(ctx context.Context, _ map[string]any) (ToolResult, error) {
			probe.enter()
			defer probe.leave()
			probe.waitForBatch(ctx)
			return ToolResult{Output: map[string]any{"tool": name}}, nil
		}}); err != nil {
			t.Fatal(err)
		}
	}
	modelClient := &parallelBatchModel{}
	runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
		return ModelBuildResult{Client: modelClient, Capabilities: []string{"text", "tool_calling", "stream"}}, nil
	})
	var events []Event
	result, err := runner.RunStream(context.Background(), Request{
		ExecutionID: "exec-parallel", ActorID: "actor-1", ScopeID: "scope-1", Task: "collect independent facts",
		Budget:          Budget{MaxIterations: 3, MaxToolCalls: 4, MaxTokens: 1000, MaxOutputBytes: 4096, Timeout: 2 * time.Second},
		ResolvedContext: &ResolvedContext{Tools: []ToolDefinition{{Name: "lookup_one", Source: "test", ResourceID: "resource-1"}, {Name: "lookup_two", Source: "test", ResourceID: "resource-1"}}},
		ToolGateway:     NewPolicyGateway(registry, nil, time.Second, 4096, 4),
	}, func(event Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("RunStream() error = %v", err)
	}
	if result.Output != "已汇总并行观察。" || result.ToolCallCount != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	probe.mu.Lock()
	maxConcurrent := probe.max
	probe.mu.Unlock()
	if maxConcurrent < 2 {
		t.Fatalf("independent tools were not run concurrently: max=%d", maxConcurrent)
	}
	resumed := make([]Event, 0, 1)
	completed := make([]int, 0, 2)
	for index, event := range events {
		switch event.Type {
		case "tool.completed":
			completed = append(completed, index)
		case "model.resumed":
			resumed = append(resumed, event)
		}
	}
	if len(completed) != 2 || len(resumed) != 1 {
		t.Fatalf("tool batch events: completed=%d resumed=%d events=%+v", len(completed), len(resumed), events)
	}
	resumedIndex := -1
	for index, event := range events {
		if event.Type == "model.resumed" {
			resumedIndex = index
			break
		}
	}
	if resumedIndex < completed[0] || resumedIndex < completed[1] {
		t.Fatalf("model.resumed preceded a tool completion: events=%+v", events)
	}
	if resumed[0].Payload["parallel"] != true || resumed[0].Payload["tool_count"] != 2 {
		t.Fatalf("model.resumed did not describe the complete parallel batch: %+v", resumed[0].Payload)
	}
	observations, ok := resumed[0].Payload["observations"].([]map[string]any)
	if !ok || len(observations) != 2 {
		t.Fatalf("parallel observations=%T %+v", resumed[0].Payload["observations"], resumed[0].Payload["observations"])
	}
	modelClient.mu.Lock()
	defer modelClient.mu.Unlock()
	if len(modelClient.requests) != 2 {
		t.Fatalf("model calls=%d, want 2", len(modelClient.requests))
	}
}

type usageTestModel struct{ turns int }

func (m *usageTestModel) Name() string { return "usage-test-model" }

func (m *usageTestModel) GenerateContent(_ context.Context, _ *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		m.turns++
		if m.turns == 1 {
			if stream {
				yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "阶段性文本"}}}, Partial: true, UsageMetadata: &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: 5, CandidatesTokenCount: 5, TotalTokenCount: 10}}, nil)
			}
			yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "准备查询"}, {FunctionCall: &genai.FunctionCall{ID: "usage-call", Name: "lookup", Args: map[string]any{}}}}}, UsageMetadata: &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: 10, CandidatesTokenCount: 10, TotalTokenCount: 20}}, nil)
			return
		}
		yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "第二轮回答"}}}, UsageMetadata: &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: 20, CandidatesTokenCount: 10, TotalTokenCount: 30}}, nil)
	}
}

func TestAgentRunnerCountsUsageOncePerModelTurn(t *testing.T) {
	modelClient := &usageTestModel{}
	registry := NewToolRegistry()
	if err := registry.Register("resource-1", ToolFunc{Def: ToolDefinition{Name: "lookup", Source: "test", ResourceID: "resource-1"}, Fn: func(context.Context, map[string]any) (ToolResult, error) {
		return ToolResult{Output: map[string]any{"ok": true}}, nil
	}}); err != nil {
		t.Fatal(err)
	}
	runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
		return ModelBuildResult{Client: modelClient, Capabilities: []string{"text", "stream"}}, nil
	})
	result, err := runner.RunStream(context.Background(), Request{
		ExecutionID: "exec-usage", ActorID: "actor-1", ScopeID: "scope-1", Task: "answer", Stream: true,
		Budget:          Budget{MaxIterations: 2, MaxTokens: 100, MaxOutputBytes: 4096},
		ResolvedContext: &ResolvedContext{Tools: []ToolDefinition{{Name: "lookup", Source: "test", ResourceID: "resource-1"}}},
		ToolGateway:     NewPolicyGateway(registry, nil, 0, 4096, 1),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalTokens != 50 {
		t.Fatalf("total tokens = %d, want 50", result.TotalTokens)
	}
}

type correctionTestModel struct{ turns int }

func (m *correctionTestModel) Name() string { return "correction-test-model" }

func (m *correctionTestModel) GenerateContent(_ context.Context, request *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		m.turns++
		if m.turns == 1 {
			yield(&model.LLMResponse{Content: &genai.Content{
				Role:  genai.RoleModel,
				Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{ID: "bad-1", Name: "failing_lookup", Args: map[string]any{}}}},
			}}, nil)
			return
		}
		if m.turns == 2 {
			yield(&model.LLMResponse{Content: &genai.Content{
				Role: genai.RoleModel,
				Parts: []*genai.Part{
					{Text: "第一次工具失败，改用备用查询。"},
					{FunctionCall: &genai.FunctionCall{ID: "good-1", Name: "fallback_lookup", Args: map[string]any{}}},
				},
			}}, nil)
			return
		}
		yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "备用查询成功，已完成诊断。"}}}}, nil)
		_ = request
	}
}

func TestAgentRunnerContinuesAfterToolFailure(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register("resource-1", ToolFunc{Def: ToolDefinition{Name: "failing_lookup", Source: "test", ResourceID: "resource-1"}, Fn: func(context.Context, map[string]any) (ToolResult, error) {
		return ToolResult{}, errors.New("upstream unavailable")
	}}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("resource-1", ToolFunc{Def: ToolDefinition{Name: "fallback_lookup", Source: "test", ResourceID: "resource-1"}, Fn: func(context.Context, map[string]any) (ToolResult, error) {
		return ToolResult{Output: map[string]any{"ok": true}}, nil
	}}); err != nil {
		t.Fatal(err)
	}
	modelClient := &correctionTestModel{}
	gateway := NewPolicyGateway(registry, nil, 0, 4096, 1)
	var events []Event
	runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
		return ModelBuildResult{Client: modelClient, Capabilities: []string{"text", "tool_calling", "stream"}}, nil
	})
	result, err := runner.RunStream(context.Background(), Request{
		ExecutionID: "exec-correction", ActorID: "actor-1", ScopeID: "scope-1", Task: "diagnose", Stream: true,
		Budget:          Budget{MaxIterations: 5, MaxToolCalls: 5, MaxTokens: 1000, MaxOutputBytes: 4096},
		ResolvedContext: &ResolvedContext{Tools: []ToolDefinition{{Name: "failing_lookup", Source: "test", ResourceID: "resource-1"}, {Name: "fallback_lookup", Source: "test", ResourceID: "resource-1"}}},
		ToolGateway:     gateway,
	}, func(event Event) error { events = append(events, event); return nil })
	if err != nil || result.Output != "备用查询成功，已完成诊断。" || result.ToolCallCount != 2 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	failed, resumedError, secondModel := false, false, 0
	for _, event := range events {
		switch event.Type {
		case "tool.failed":
			failed = true
		case "model.resumed":
			if event.Payload["outcome"] == "error" {
				resumedError = true
			}
		case "model.started":
			secondModel++
		}
	}
	if !failed || !resumedError || secondModel < 3 {
		t.Fatalf("missing correction loop events: failed=%v resumedError=%v modelTurns=%d events=%+v", failed, resumedError, secondModel, events)
	}
}

type unknownToolModel struct{ turns int }

func (m *unknownToolModel) Name() string { return "unknown-tool-model" }

func (m *unknownToolModel) GenerateContent(_ context.Context, _ *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		m.turns++
		if m.turns == 1 {
			yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{ID: "unknown-1", Name: "not_registered", Args: map[string]any{}}}}}}, nil)
			return
		}
		yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "工具不存在，已改为直接说明限制。"}}}}, nil)
	}
}

func TestAgentRunnerSurfacesUnknownToolObservation(t *testing.T) {
	modelClient := &unknownToolModel{}
	var events []Event
	audit := &memoryToolCallStore{}
	registry := NewToolRegistry()
	if err := registry.Register("resource-1", ToolFunc{Def: ToolDefinition{Name: "known", Source: "test", ResourceID: "resource-1"}, Fn: func(context.Context, map[string]any) (ToolResult, error) {
		return ToolResult{Output: map[string]any{"ok": true}}, nil
	}}); err != nil {
		t.Fatal(err)
	}
	gateway := NewPolicyGateway(registry, nil, 0, 4096, 1)
	gateway.AuditStore = audit
	runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
		return ModelBuildResult{Client: modelClient, Capabilities: []string{"text", "tool_calling", "stream"}}, nil
	})
	result, err := runner.RunStream(context.Background(), Request{
		ExecutionID: "exec-unknown-tool", ActorID: "actor-1", ScopeID: "scope-1", Task: "answer",
		Budget:          Budget{MaxIterations: 3, MaxTokens: 1000, MaxOutputBytes: 4096},
		ResolvedContext: &ResolvedContext{Tools: []ToolDefinition{{Name: "known", Source: "test", ResourceID: "resource-1"}}},
		ToolGateway:     gateway,
	}, func(event Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil || result.Output != "工具不存在，已改为直接说明限制。" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	failed := false
	resumed := false
	for _, event := range events {
		if event.Type == "tool.failed" && event.Payload["error_code"] == "tool_not_found" {
			failed = true
		}
		if event.Type == "model.resumed" && event.Payload["outcome"] == "error" && event.Payload["tool"] == "not_registered" {
			if !failed {
				t.Fatalf("unknown tool observation arrived before tool.failed: %+v", events)
			}
			resumed = true
		}
	}
	if len(audit.records) != 1 || audit.records[0].ErrorCode != "tool_not_found" || audit.records[0].ToolName != "not_registered" {
		t.Fatalf("unknown tool audit=%+v", audit.records)
	}
	if !resumed {
		t.Fatalf("unknown tool observation was not surfaced: %+v", events)
	}
}

type validationTestModel struct{ turns int }

func (m *validationTestModel) Name() string { return "validation-test-model" }

func (m *validationTestModel) GenerateContent(_ context.Context, _ *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		m.turns++
		if m.turns == 1 {
			yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{ID: "invalid-1", Name: "lookup", Args: map[string]any{}}}}}}, nil)
			return
		}
		yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "参数无效，已说明限制。"}}}}, nil)
	}
}

func TestAgentRunnerEmitsLifecycleForToolArgumentValidation(t *testing.T) {
	modelClient := &validationTestModel{}
	registry := NewToolRegistry()
	if err := registry.Register("resource-1", ToolFunc{Def: ToolDefinition{Name: "lookup", Source: "test", ResourceID: "resource-1"}, Fn: func(context.Context, map[string]any) (ToolResult, error) {
		return ToolResult{Output: map[string]any{"unexpected": true}}, nil
	}}); err != nil {
		t.Fatal(err)
	}
	runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
		return ModelBuildResult{Client: modelClient, Capabilities: []string{"text", "tool_calling", "stream"}}, nil
	})
	var events []Event
	result, err := runner.RunStream(context.Background(), Request{
		ExecutionID: "exec-validation", ActorID: "actor-1", ScopeID: "scope-1", Task: "answer",
		Budget:          Budget{MaxIterations: 3, MaxToolCalls: 3, MaxTokens: 1000, MaxOutputBytes: 4096},
		ResolvedContext: &ResolvedContext{Tools: []ToolDefinition{{Name: "lookup", ResourceID: "resource-1", InputSchema: json.RawMessage(`{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`)}}},
		ToolGateway:     NewPolicyGateway(registry, nil, 0, 4096, 1),
	}, func(event Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil || result.Output != "参数无效，已说明限制。" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	failed, resumed := false, false
	for _, event := range events {
		if event.Type == "tool.failed" && event.Payload["error_code"] == "tool_validation" {
			failed = true
		}
		if event.Type == "model.resumed" && event.Payload["outcome"] == "error" && event.Payload["tool"] == "lookup" {
			resumed = true
		}
	}
	if !failed || !resumed {
		t.Fatalf("validation failure was not surfaced: failed=%v resumed=%v events=%+v", failed, resumed, events)
	}
}

type emptyAnswerModel struct{}

func (emptyAnswerModel) Name() string { return "empty-answer-model" }
func (emptyAnswerModel) GenerateContent(context.Context, *model.LLMRequest, bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel}}, nil)
	}
}

func TestAgentRunnerRejectsEmptyFinalAnswer(t *testing.T) {
	runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
		return ModelBuildResult{Client: emptyAnswerModel{}, Capabilities: []string{"text"}}, nil
	})
	result, err := runner.Run(context.Background(), Request{ExecutionID: "exec-empty", ActorID: "actor-1", ScopeID: "scope-1", Task: "answer", Budget: Budget{MaxIterations: 1, MaxTokens: 100, MaxOutputBytes: 4096}})
	if err == nil || result.Status != StatusFailed || result.ErrorCode != "empty_output" {
		t.Fatalf("result=%+v err=%v, want empty_output failure", result, err)
	}
}

func TestObservationSummaryRedactsAndBoundsOutput(t *testing.T) {
	const logSize = 6000
	summary := summarizeObservation(map[string]any{
		"password": "do-not-show",
		"nested":   map[string]any{"api_key": "also-secret"},
		"logs":     strings.Repeat("x", logSize),
	})
	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > maxObservationBytes || strings.Contains(string(encoded), "do-not-show") || strings.Contains(string(encoded), "also-secret") {
		t.Fatalf("unsafe observation summary: len=%d value=%s", len(encoded), encoded)
	}
	if object, ok := summary.(map[string]any); ok {
		if logs, ok := object["logs"].(string); !ok || len(logs) != logSize {
			t.Fatalf("long log field was unexpectedly truncated: type=%T len=%d", object["logs"], len(logs))
		}
	} else {
		t.Fatalf("observation summary type=%T, want object", summary)
	}
}

func TestAgentRunnerRunUsesRequestEventSinkForProgress(t *testing.T) {
	modelClient := &loopTestModel{}
	var events []Event
	runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
		return ModelBuildResult{Client: modelClient, Capabilities: []string{"text"}}, nil
	})
	_, err := runner.Run(context.Background(), Request{
		ExecutionID: "exec-non-stream", ActorID: "actor-1", ScopeID: "scope-1", Task: "answer",
		Budget:    Budget{MaxIterations: 2, MaxTokens: 1000, MaxOutputBytes: 4096},
		EventSink: func(event Event) error { events = append(events, event); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.Type == "assistant.completed" {
			return
		}
	}
	t.Fatalf("request event sink did not receive assistant.completed: %+v", events)
}

type secretAnswerModel struct{}

func (secretAnswerModel) Name() string { return "secret-answer-model" }
func (secretAnswerModel) GenerateContent(context.Context, *model.LLMRequest, bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: `{"answer":"token=super-secret"}`}}}}, nil)
	}
}

func TestAgentRunnerRedactsReturnedAnswer(t *testing.T) {
	runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
		return ModelBuildResult{Client: secretAnswerModel{}, Capabilities: []string{"text"}}, nil
	})
	result, err := runner.Run(context.Background(), Request{ExecutionID: "exec-secret-answer", ActorID: "actor-1", ScopeID: "scope-1", Task: "answer", Budget: Budget{MaxIterations: 1, MaxTokens: 100, MaxOutputBytes: 4096}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.Output, "super-secret") || !strings.Contains(result.Output, "[REDACTED]") {
		t.Fatalf("returned answer was not redacted: %q", result.Output)
	}
}

func TestScrubVisibleTextPreservesStructuredJSON(t *testing.T) {
	input := `{"token":"super-secret","count":1,"nested":{"api_key":"another-secret"}}`
	redacted := scrubVisibleText(input)
	if strings.Contains(redacted, "super-secret") || strings.Contains(redacted, "another-secret") {
		t.Fatalf("structured secret was not redacted: %s", redacted)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(redacted), &decoded); err != nil {
		t.Fatalf("redacted structured output became invalid JSON: %v (%s)", err, redacted)
	}
}

func TestScrubVisibleTextRedactsBearerAndKeyVariants(t *testing.T) {
	input := "Authorization: Bearer super-secret private_key=private-secret access-key=access-secret"
	redacted := scrubVisibleText(input)
	for _, secret := range []string{"super-secret", "private-secret", "access-secret"} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("secret %q leaked: %s", secret, redacted)
		}
	}
}

func TestExecutionPromptIncludesSafeEnvironmentFacts(t *testing.T) {
	prompt, err := executionPrompt(Request{
		Task: "检查资源",
		ResolvedContext: &ResolvedContext{Facts: []ContextFact{{
			ResourceID: "resource-1",
			Kind:       "PostgreSQL",
			Summary:    map[string]any{"status": "healthy"},
			Data:       map[string]any{"password": "do-not-send", "connections": 3},
			Untrusted:  true,
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "environment_facts") || !strings.Contains(prompt, "healthy") {
		t.Fatalf("environment facts were not included in prompt: %s", prompt)
	}
	if strings.Contains(prompt, "do-not-send") {
		t.Fatalf("environment fact secret leaked into prompt: %s", prompt)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(prompt), &decoded); err != nil {
		t.Fatalf("execution prompt is not valid JSON: %v", err)
	}
}
