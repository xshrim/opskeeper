package aiengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrToolNotFound = errors.New("AIEngine tool was not found")
	ErrToolDenied   = errors.New("AIEngine tool call was denied")
	ErrToolInvalid  = errors.New("AIEngine tool request is invalid")
	ErrToolLimited  = errors.New("AIEngine tool response exceeds the configured limit")
)

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
	Source      string          `json:"source,omitempty"`
	ResourceID  string          `json:"resource_id,omitempty"`
	ReadOnly    bool            `json:"read_only"`
}

type ToolResult struct {
	Output    any  `json:"output,omitempty"`
	Untrusted bool `json:"untrusted"`
	Truncated bool `json:"truncated,omitempty"`
}

type Tool interface {
	Definition() ToolDefinition
	Invoke(context.Context, map[string]any) (ToolResult, error)
}

type ToolFunc struct {
	Def ToolDefinition
	Fn  func(context.Context, map[string]any) (ToolResult, error)
}

func (t ToolFunc) Definition() ToolDefinition { return t.Def }
func (t ToolFunc) Invoke(ctx context.Context, arguments map[string]any) (ToolResult, error) {
	if t.Fn == nil {
		return ToolResult{}, ErrToolInvalid
	}
	return t.Fn(ctx, arguments)
}

type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]map[string]Tool
}

func NewToolRegistry() *ToolRegistry { return &ToolRegistry{tools: make(map[string]map[string]Tool)} }

func (r *ToolRegistry) Register(resourceID string, tool Tool) error {
	return r.register(resourceID, tool, false)
}

// Upsert replaces a resource's tool with the same name. Context providers are
// resolved for every execution and may refresh MCP metadata, so registration
// must be safe when a shared Runtime resolves the same resource repeatedly.
func (r *ToolRegistry) Upsert(resourceID string, tool Tool) error {
	return r.register(resourceID, tool, true)
}

func (r *ToolRegistry) register(resourceID string, tool Tool, replace bool) error {
	if r == nil || tool == nil {
		return ErrToolInvalid
	}
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return fmt.Errorf("%w: resource id is required", ErrToolInvalid)
	}
	definition := tool.Definition()
	definition.Name = strings.TrimSpace(definition.Name)
	if definition.Name == "" || strings.TrimSpace(definition.Source) == "" {
		return fmt.Errorf("%w: tool name and source are required", ErrToolInvalid)
	}
	if definition.ResourceID == "" {
		definition.ResourceID = resourceID
	}
	if definition.ResourceID != resourceID {
		return fmt.Errorf("%w: tool resource does not match registration resource", ErrToolInvalid)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tools[resourceID] == nil {
		r.tools[resourceID] = make(map[string]Tool)
	}
	if _, exists := r.tools[resourceID][definition.Name]; exists && !replace {
		return fmt.Errorf("%w: tool %s is already registered", ErrToolInvalid, definition.Name)
	}
	if normalized, ok := tool.(ToolFunc); ok {
		normalized.Def = definition
		tool = normalized
	}
	r.tools[resourceID][definition.Name] = tool
	return nil
}

func (r *ToolRegistry) Get(resourceID, name string) (Tool, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[strings.TrimSpace(resourceID)][strings.TrimSpace(name)]
	return tool, ok
}

func (r *ToolRegistry) List(resourceID string) []ToolDefinition {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]ToolDefinition, 0, len(r.tools[resourceID]))
	for _, tool := range r.tools[strings.TrimSpace(resourceID)] {
		items = append(items, tool.Definition())
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

type ToolCall struct {
	ExecutionID        string
	Sequence           int
	ActorID            string
	ScopeID            string
	ResourceID         string
	Name               string
	ProviderResourceID string
	ModelName          string
	Arguments          map[string]any
	EventSink          EventSink
}

type ToolAuthorizer func(context.Context, ToolCall, ToolDefinition) error

type PolicyGateway struct {
	Registry         *ToolRegistry
	Authorize        ToolAuthorizer
	Timeout          time.Duration
	MaxResponseBytes int64
	MaxConcurrent    int
	AuditStore       ToolCallStore
	semaphore        chan struct{}
	semaphoreOnce    sync.Once
	toolSequence     atomic.Int64
}

func NewPolicyGateway(registry *ToolRegistry, authorize ToolAuthorizer, timeout time.Duration, maxResponseBytes int64, maxConcurrent int) *PolicyGateway {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if maxResponseBytes <= 0 {
		maxResponseBytes = 4 << 20
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 8
	}
	gateway := &PolicyGateway{Registry: registry, Authorize: authorize, Timeout: timeout, MaxResponseBytes: maxResponseBytes, MaxConcurrent: maxConcurrent}
	gateway.semaphoreOnce.Do(func() { gateway.semaphore = make(chan struct{}, maxConcurrent) })
	return gateway
}

func (g *PolicyGateway) Invoke(ctx context.Context, call ToolCall) (ToolResult, error) {
	if g == nil || g.Registry == nil {
		return ToolResult{}, ErrToolInvalid
	}
	g.semaphoreOnce.Do(func() {
		if g.Timeout <= 0 {
			g.Timeout = 10 * time.Second
		}
		if g.MaxResponseBytes <= 0 {
			g.MaxResponseBytes = 4 << 20
		}
		if g.MaxConcurrent <= 0 {
			g.MaxConcurrent = 8
		}
		g.semaphore = make(chan struct{}, g.MaxConcurrent)
	})
	call.ResourceID = strings.TrimSpace(call.ResourceID)
	call.Name = strings.TrimSpace(call.Name)
	if call.Sequence <= 0 {
		call.Sequence = int(g.toolSequence.Add(1))
	}
	if call.ResourceID == "" || call.Name == "" {
		return ToolResult{}, fmt.Errorf("%w: resource_id and tool name are required", ErrToolInvalid)
	}
	tool, ok := g.Registry.Get(call.ResourceID, call.Name)
	if !ok {
		return ToolResult{}, fmt.Errorf("%w: %s", ErrToolNotFound, call.Name)
	}
	definition := tool.Definition()
	if g.Authorize != nil {
		if err := g.Authorize(ctx, call, definition); err != nil {
			return ToolResult{}, fmt.Errorf("%w: %v", ErrToolDenied, err)
		}
	}
	select {
	case g.semaphore <- struct{}{}:
		defer func() { <-g.semaphore }()
	case <-ctx.Done():
		return ToolResult{}, ctx.Err()
	}
	started := time.Now().UTC()
	if err := emitToolEvent(call.EventSink, Event{Type: "tool.requested", Status: StatusRunning, Payload: map[string]any{"tool": definition.Name, "resource_id": call.ResourceID, "arguments": redactValue(call.Arguments)}}); err != nil {
		return ToolResult{}, err
	}
	if err := emitToolEvent(call.EventSink, Event{Type: "tool.started", Status: StatusRunning, Payload: map[string]any{"tool": definition.Name, "resource_id": call.ResourceID, "arguments": redactValue(call.Arguments)}}); err != nil {
		return ToolResult{}, err
	}
	runCtx, cancel := context.WithTimeout(ctx, g.Timeout)
	defer cancel()
	result, err := tool.Invoke(runCtx, call.Arguments)
	completedAt := time.Now().UTC()
	durationMS := completedAt.Sub(started).Milliseconds()
	if err != nil {
		if g.AuditStore != nil {
			_ = g.AuditStore.RecordToolCall(context.Background(), ToolCallRecord{ExecutionID: call.ExecutionID, Sequence: call.Sequence, ProviderResourceID: call.ProviderResourceID, ModelName: call.ModelName, ResourceID: call.ResourceID, ToolName: definition.Name, Arguments: call.Arguments, Status: StatusFailed, ErrorCode: "tool_execution", Error: err.Error(), StartedAt: started, CompletedAt: completedAt, DurationMS: durationMS})
		}
		_ = emitToolEvent(call.EventSink, Event{Type: "tool.failed", Status: StatusFailed, Payload: map[string]any{"tool": definition.Name, "resource_id": call.ResourceID, "arguments": redactValue(call.Arguments), "output": map[string]any{"error": err.Error()}, "error": err.Error(), "duration_ms": time.Since(started).Milliseconds()}})
		return ToolResult{}, err
	}
	encoded, marshalErr := json.Marshal(result.Output)
	if marshalErr != nil {
		if g.AuditStore != nil {
			_ = g.AuditStore.RecordToolCall(context.Background(), ToolCallRecord{ExecutionID: call.ExecutionID, Sequence: call.Sequence, ProviderResourceID: call.ProviderResourceID, ModelName: call.ModelName, ResourceID: call.ResourceID, ToolName: definition.Name, Arguments: call.Arguments, Status: StatusFailed, ErrorCode: "output_not_serializable", Error: marshalErr.Error(), StartedAt: started, CompletedAt: completedAt, DurationMS: durationMS})
		}
		_ = emitToolEvent(call.EventSink, Event{Type: "tool.failed", Status: StatusFailed, Payload: map[string]any{
			"tool": definition.Name, "resource_id": call.ResourceID,
			"arguments": redactValue(call.Arguments),
			"output":    map[string]any{"error": marshalErr.Error()},
			"error":     marshalErr.Error(), "duration_ms": time.Since(started).Milliseconds(),
		}})
		return ToolResult{}, fmt.Errorf("%w: output is not JSON serializable: %v", ErrToolInvalid, marshalErr)
	}
	if int64(len(encoded)) > g.MaxResponseBytes {
		if g.AuditStore != nil {
			_ = g.AuditStore.RecordToolCall(context.Background(), ToolCallRecord{ExecutionID: call.ExecutionID, Sequence: call.Sequence, ProviderResourceID: call.ProviderResourceID, ModelName: call.ModelName, ResourceID: call.ResourceID, ToolName: definition.Name, Arguments: call.Arguments, Status: StatusFailed, ErrorCode: "response_too_large", Error: ErrToolLimited.Error(), StartedAt: started, CompletedAt: completedAt, DurationMS: durationMS})
		}
		_ = emitToolEvent(call.EventSink, Event{Type: "tool.failed", Status: StatusFailed, Payload: map[string]any{"tool": definition.Name, "resource_id": call.ResourceID, "arguments": redactValue(call.Arguments), "output": map[string]any{"error": ErrToolLimited.Error()}, "error": ErrToolLimited.Error(), "duration_ms": time.Since(started).Milliseconds()}})
		return ToolResult{}, ErrToolLimited
	}
	if g.AuditStore != nil {
		_ = g.AuditStore.RecordToolCall(context.Background(), ToolCallRecord{ExecutionID: call.ExecutionID, Sequence: call.Sequence, ProviderResourceID: call.ProviderResourceID, ModelName: call.ModelName, ResourceID: call.ResourceID, ToolName: definition.Name, Arguments: call.Arguments, Output: result.Output, Status: StatusSucceeded, StartedAt: started, CompletedAt: completedAt, DurationMS: durationMS})
	}
	if err := emitToolEvent(call.EventSink, Event{Type: "tool.completed", Status: StatusSucceeded, Payload: map[string]any{"tool": definition.Name, "resource_id": call.ResourceID, "arguments": redactValue(call.Arguments), "output": redactValue(result.Output), "duration_ms": time.Since(started).Milliseconds()}}); err != nil {
		return ToolResult{}, err
	}
	return result, nil
}

func emitToolEvent(sink EventSink, event Event) error {
	if sink == nil {
		return nil
	}
	return sink(event)
}
