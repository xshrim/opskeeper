package aiengine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	ExecutionID string
	Sequence    int
	Iteration   int
	CallID      string
	ActorID     string
	ScopeID     string
	ResourceID  string
	Name        string
	// ModelToolName is the name exposed to the model. It differs from Name
	// when two selected resources expose the same MCP/connector tool name. The
	// gateway and audit trail always use Name, while the alias keeps the ADK
	// function declaration set globally unique.
	ModelToolName      string
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

func (g *PolicyGateway) recordToolCall(record ToolCallRecord) {
	if g == nil || g.AuditStore == nil {
		return
	}
	// Audit failures must not turn a provider/tool failure into a successful
	// execution, but the gateway deliberately keeps the execution path
	// available when the optional audit sink is unavailable.
	_ = g.AuditStore.RecordToolCall(context.Background(), record)
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
		err := fmt.Errorf("%w: resource_id and tool name are required", ErrToolInvalid)
		message := publicError(err)
		at := time.Now().UTC()
		_ = emitToolEvent(call.EventSink, Event{Type: "tool.requested", Status: StatusRunning, Payload: toolEventPayload(call, call.Name, map[string]any{"arguments": redactValue(call.Arguments)})})
		_ = emitToolEvent(call.EventSink, Event{Type: "tool.failed", Status: StatusFailed, Payload: toolEventPayload(call, call.Name, map[string]any{
			"error":       message,
			"error_code":  "tool_validation",
			"output":      map[string]any{"error": message},
			"duration_ms": int64(0),
		})})
		g.recordToolCall(ToolCallRecord{ExecutionID: call.ExecutionID, Sequence: call.Sequence, ProviderResourceID: call.ProviderResourceID, ModelName: call.ModelName, ResourceID: call.ResourceID, ToolName: call.Name, Arguments: call.Arguments, Status: StatusFailed, ErrorCode: "tool_validation", Error: message, StartedAt: at, CompletedAt: at})
		return ToolResult{}, err
	}
	tool, ok := g.Registry.Get(call.ResourceID, call.Name)
	if !ok {
		err := fmt.Errorf("%w: %s", ErrToolNotFound, call.Name)
		message := publicError(err)
		at := time.Now().UTC()
		g.recordToolCall(ToolCallRecord{ExecutionID: call.ExecutionID, Sequence: call.Sequence, ProviderResourceID: call.ProviderResourceID, ModelName: call.ModelName, ResourceID: call.ResourceID, ToolName: call.Name, Arguments: call.Arguments, Status: StatusFailed, ErrorCode: "tool_not_found", Error: message, StartedAt: at, CompletedAt: at})
		// Keep registry races and stale context definitions visible in the
		// same requested -> failed lifecycle as ordinary tool invocations.
		_ = emitToolEvent(call.EventSink, Event{Type: "tool.requested", Status: StatusRunning, Payload: toolEventPayload(call, call.Name, map[string]any{"arguments": redactValue(call.Arguments)})})
		_ = emitToolEvent(call.EventSink, Event{Type: "tool.failed", Status: StatusFailed, Payload: toolEventPayload(call, call.Name, map[string]any{
			"error":       message,
			"error_code":  "tool_not_found",
			"output":      map[string]any{"error": message},
			"duration_ms": int64(0),
		})})
		return ToolResult{}, err
	}
	definition := tool.Definition()
	if err := emitToolEvent(call.EventSink, Event{Type: "tool.requested", Status: StatusRunning, Payload: toolEventPayload(call, definition.Name, map[string]any{"arguments": redactValue(call.Arguments)})}); err != nil {
		return ToolResult{}, err
	}
	if g.Authorize != nil {
		if err := g.Authorize(ctx, call, definition); err != nil {
			denied := fmt.Errorf("%w: %v", ErrToolDenied, err)
			message := publicError(denied)
			at := time.Now().UTC()
			_ = emitToolEvent(call.EventSink, Event{Type: "tool.failed", Status: StatusFailed, Payload: toolEventPayload(call, definition.Name, map[string]any{
				"error":       message,
				"error_code":  "tool_denied",
				"output":      map[string]any{"error": message},
				"duration_ms": int64(0),
			})})
			g.recordToolCall(ToolCallRecord{ExecutionID: call.ExecutionID, Sequence: call.Sequence, ProviderResourceID: call.ProviderResourceID, ModelName: call.ModelName, ResourceID: call.ResourceID, ToolName: definition.Name, Arguments: call.Arguments, Status: StatusFailed, ErrorCode: "tool_denied", Error: message, StartedAt: at, CompletedAt: at})
			return ToolResult{}, denied
		}
	}
	select {
	case g.semaphore <- struct{}{}:
		defer func() { <-g.semaphore }()
	case <-ctx.Done():
		message := publicError(ctx.Err())
		at := time.Now().UTC()
		g.recordToolCall(ToolCallRecord{ExecutionID: call.ExecutionID, Sequence: call.Sequence, ProviderResourceID: call.ProviderResourceID, ModelName: call.ModelName, ResourceID: call.ResourceID, ToolName: definition.Name, Arguments: call.Arguments, Status: StatusFailed, ErrorCode: "cancelled", Error: message, StartedAt: at, CompletedAt: at})
		_ = emitToolEvent(call.EventSink, Event{Type: "tool.failed", Status: StatusFailed, Payload: toolEventPayload(call, definition.Name, map[string]any{
			"error":       message,
			"error_code":  "cancelled",
			"output":      map[string]any{"error": message},
			"duration_ms": int64(0),
		})})
		return ToolResult{}, ctx.Err()
	}
	started := time.Now().UTC()
	if err := emitToolEvent(call.EventSink, Event{Type: "tool.started", Status: StatusRunning, Payload: toolEventPayload(call, definition.Name, map[string]any{"arguments": redactValue(call.Arguments)})}); err != nil {
		return ToolResult{}, err
	}
	runCtx, cancel := context.WithTimeout(ctx, g.Timeout)
	defer cancel()
	result, err := tool.Invoke(runCtx, call.Arguments)
	completedAt := time.Now().UTC()
	durationMS := completedAt.Sub(started).Milliseconds()
	if err != nil {
		message := publicError(err)
		g.recordToolCall(ToolCallRecord{ExecutionID: call.ExecutionID, Sequence: call.Sequence, ProviderResourceID: call.ProviderResourceID, ModelName: call.ModelName, ResourceID: call.ResourceID, ToolName: definition.Name, Arguments: call.Arguments, Status: StatusFailed, ErrorCode: "tool_execution", Error: message, StartedAt: started, CompletedAt: completedAt, DurationMS: durationMS})
		_ = emitToolEvent(call.EventSink, Event{Type: "tool.failed", Status: StatusFailed, Payload: toolEventPayload(call, definition.Name, map[string]any{"arguments": redactValue(call.Arguments), "output": map[string]any{"error": message}, "error": message, "duration_ms": time.Since(started).Milliseconds()})})
		return ToolResult{}, err
	}
	encoded, marshalErr := json.Marshal(result.Output)
	if marshalErr != nil {
		message := publicError(marshalErr)
		g.recordToolCall(ToolCallRecord{ExecutionID: call.ExecutionID, Sequence: call.Sequence, ProviderResourceID: call.ProviderResourceID, ModelName: call.ModelName, ResourceID: call.ResourceID, ToolName: definition.Name, Arguments: call.Arguments, Status: StatusFailed, ErrorCode: "output_not_serializable", Error: message, StartedAt: started, CompletedAt: completedAt, DurationMS: durationMS})
		_ = emitToolEvent(call.EventSink, Event{Type: "tool.failed", Status: StatusFailed, Payload: toolEventPayload(call, definition.Name, map[string]any{
			"arguments": redactValue(call.Arguments),
			"output":    map[string]any{"error": message},
			"error":     message, "duration_ms": time.Since(started).Milliseconds(),
		})})
		return ToolResult{}, fmt.Errorf("%w: output is not JSON serializable: %v", ErrToolInvalid, marshalErr)
	}
	if int64(len(encoded)) > g.MaxResponseBytes {
		message := publicError(ErrToolLimited)
		g.recordToolCall(ToolCallRecord{ExecutionID: call.ExecutionID, Sequence: call.Sequence, ProviderResourceID: call.ProviderResourceID, ModelName: call.ModelName, ResourceID: call.ResourceID, ToolName: definition.Name, Arguments: call.Arguments, Status: StatusFailed, ErrorCode: "response_too_large", Error: message, StartedAt: started, CompletedAt: completedAt, DurationMS: durationMS})
		_ = emitToolEvent(call.EventSink, Event{Type: "tool.failed", Status: StatusFailed, Payload: toolEventPayload(call, definition.Name, map[string]any{"arguments": redactValue(call.Arguments), "output": map[string]any{"error": message}, "error": message, "duration_ms": time.Since(started).Milliseconds()})})
		return ToolResult{}, ErrToolLimited
	}
	g.recordToolCall(ToolCallRecord{ExecutionID: call.ExecutionID, Sequence: call.Sequence, ProviderResourceID: call.ProviderResourceID, ModelName: call.ModelName, ResourceID: call.ResourceID, ToolName: definition.Name, Arguments: call.Arguments, Output: result.Output, Status: StatusSucceeded, StartedAt: started, CompletedAt: completedAt, DurationMS: durationMS})
	if err := emitToolEvent(call.EventSink, Event{Type: "tool.completed", Status: StatusSucceeded, Payload: toolEventPayload(call, definition.Name, map[string]any{"arguments": redactValue(call.Arguments), "output": summarizeObservation(result.Output), "duration_ms": time.Since(started).Milliseconds()})}); err != nil {
		return ToolResult{}, err
	}
	return result, nil
}

func toolEventPayload(call ToolCall, name string, extra map[string]any) map[string]any {
	payload := map[string]any{"tool": name, "resource_id": call.ResourceID}
	if call.ModelToolName != "" && call.ModelToolName != name {
		payload["model_tool"] = call.ModelToolName
	}
	if call.Sequence > 0 {
		payload["call_sequence"] = call.Sequence
	}
	if call.Iteration > 0 {
		payload["iteration"] = call.Iteration
	}
	if call.CallID != "" {
		payload["call_id"] = call.CallID
	}
	for key, value := range extra {
		payload[key] = value
	}
	return payload
}

// toolBinding is the execution-time mapping between the canonical tool name
// used by PolicyGateway and the globally unique name advertised to an LLM.
// MCP servers are independent resources, so the same tool name (for example
// "query_logs") can legitimately be exposed by more than one selected server.
// ADK and most model APIs require function declaration names to be unique;
// aliases are therefore generated only for collisions and remain stable for a
// given resource/name pair within an execution.
type toolBinding struct {
	definition ToolDefinition
	modelName  string
}

func buildToolBindings(definitions []ToolDefinition) []toolBinding {
	valid := make([]ToolDefinition, 0, len(definitions))
	nameCounts := make(map[string]int)
	for _, definition := range definitions {
		definition.Name = strings.TrimSpace(definition.Name)
		definition.ResourceID = strings.TrimSpace(definition.ResourceID)
		if definition.Name == "" || definition.ResourceID == "" {
			continue
		}
		valid = append(valid, definition)
		nameCounts[definition.Name]++
	}

	bindings := make([]toolBinding, 0, len(valid))
	used := make(map[string]struct{}, len(valid))
	for _, definition := range valid {
		modelName := definition.Name
		if nameCounts[definition.Name] > 1 {
			modelName = collisionToolAlias(definition.Name, definition.ResourceID)
		}
		// A malformed resolver may return the same name/resource more than once.
		// Keep the declaration set valid even in that case; the registry's
		// resource/name key still determines which implementation is invoked.
		modelName = uniqueModelToolName(modelName, used)
		used[modelName] = struct{}{}
		bindings = append(bindings, toolBinding{definition: definition, modelName: modelName})
	}
	return bindings
}

func collisionToolAlias(name, resourceID string) string {
	// Resource IDs are usually UUIDs and fit alongside the tool name. For
	// longer IDs/names use a short digest so aliases stay within common model
	// function-name limits (64 bytes) without sacrificing determinism.
	resourcePart := sanitizeToolNamePart(resourceID)
	alias := name + "__" + resourcePart
	if len(alias) <= 64 {
		return alias
	}
	digest := sha256.Sum256([]byte(resourceID))
	resourcePart = hex.EncodeToString(digest[:])[:10]
	maxName := 64 - len("__") - len(resourcePart)
	if maxName < 1 {
		maxName = 1
	}
	name = truncateASCII(name, maxName)
	return name + "__" + resourcePart
}

func uniqueModelToolName(name string, used map[string]struct{}) string {
	if _, exists := used[name]; !exists {
		return name
	}
	for index := 2; ; index++ {
		suffix := fmt.Sprintf("_%d", index)
		candidate := name
		if len(candidate)+len(suffix) > 64 {
			candidate = truncateASCII(candidate, 64-len(suffix))
		}
		candidate += suffix
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
}

func sanitizeToolNamePart(value string) string {
	var builder strings.Builder
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z', character >= 'A' && character <= 'Z', character >= '0' && character <= '9':
			builder.WriteRune(character)
		default:
			builder.WriteByte('_')
		}
	}
	if builder.Len() == 0 {
		return "resource"
	}
	return builder.String()
}

func truncateASCII(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	return value[:maxBytes]
}

func emitToolEvent(sink EventSink, event Event) error {
	if sink == nil {
		return nil
	}
	return sink(event)
}
