package tool

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

// Registry stores one resource type's tools by their stable public name.
// Resource identity and authorization are intentionally handled by the
// caller, so the same registry can be used by MCP and Direct adapters.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

type registeredTool struct {
	tool       Tool
	definition Definition
}

func (t registeredTool) Definition() Definition { return t.definition }

func (t registeredTool) Invoke(ctx context.Context, invocation Invocation) (Result, error) {
	return t.tool.Invoke(ctx, invocation)
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(tool Tool) error {
	if r == nil || tool == nil {
		return ErrInvalidTool
	}
	definition := normalizeDefinition(tool.Definition())
	if err := validateDefinition(definition); err != nil {
		return err
	}
	tool = registeredTool{tool: tool, definition: definition}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tools == nil {
		r.tools = make(map[string]Tool)
	}
	if _, exists := r.tools[definition.Name]; exists {
		return ErrToolExists
	}
	r.tools[definition.Name] = tool
	return nil
}

func (r *Registry) Upsert(tool Tool) error {
	if r == nil || tool == nil {
		return ErrInvalidTool
	}
	definition := normalizeDefinition(tool.Definition())
	if err := validateDefinition(definition); err != nil {
		return err
	}
	tool = registeredTool{tool: tool, definition: definition}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tools == nil {
		r.tools = make(map[string]Tool)
	}
	r.tools[definition.Name] = tool
	return nil
}

func (r *Registry) Get(name string) (Tool, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[strings.TrimSpace(name)]
	return tool, ok
}

func (r *Registry) List() []Definition {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]Definition, 0, len(r.tools))
	for _, tool := range r.tools {
		items = append(items, normalizeDefinition(tool.Definition()))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func (r *Registry) Invoke(ctx context.Context, name string, invocation Invocation) (Result, error) {
	tool, ok := r.Get(name)
	if !ok {
		return Result{}, NewError(CodeToolUnavailable, "invoke resource tool", ErrToolNotFound)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	invocation.Arguments = cloneArguments(invocation.Arguments)
	result, err := tool.Invoke(ctx, invocation)
	if err != nil {
		return Result{}, err
	}
	result.Warnings = append([]Warning(nil), result.Warnings...)
	return result, nil
}

func normalizeDefinition(definition Definition) Definition {
	definition.Name = strings.TrimSpace(definition.Name)
	definition.Description = strings.TrimSpace(definition.Description)
	definition.InputSchema = cloneSchema(definition.InputSchema)
	return definition
}

func validateDefinition(definition Definition) error {
	if definition.Name == "" {
		return invalidTool("tool name is required")
	}
	if strings.ContainsAny(definition.Name, " \t\r\n") {
		return invalidTool("tool name must not contain whitespace")
	}
	if len(definition.Name) > 128 {
		return invalidTool("tool name must not exceed 128 bytes")
	}
	if len(definition.InputSchema) > 0 && !json.Valid(definition.InputSchema) {
		return invalidTool("tool input schema must be valid JSON")
	}
	return nil
}
