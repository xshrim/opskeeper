package engine

import (
	"context"
	"fmt"
	"strings"
)

type ContextResource struct {
	ID       string  `json:"id"`
	ScopeID  string  `json:"scope_id"`
	Kind     string  `json:"kind"`
	Name     string  `json:"name"`
	Subtype  string  `json:"subtype,omitempty"`
	AgentRef *string `json:"agent_ref,omitempty"`
	// Binding is a server-owned relationship binding for the current execution
	// (for example a Repository application directory), not resource config.
	Binding map[string]any `json:"-"`
	Config  map[string]any `json:"-"`
}

type ContextFact struct {
	ResourceID string         `json:"resource_id"`
	Kind       string         `json:"kind"`
	Summary    map[string]any `json:"summary,omitempty"`
	Data       any            `json:"data,omitempty"`
	Untrusted  bool           `json:"untrusted"`
}

type ResolvedContext struct {
	Resources []ContextResource `json:"resources"`
	Tools     []ToolDefinition  `json:"tools"`
	Facts     []ContextFact     `json:"facts"`
	// implementations and cleanups are execution-local and deliberately not
	// serialized. They keep relationship bindings out of shared registries.
	implementations map[string]Tool
	cleanups        []func() error
}

func contextToolKey(resourceID, name string) string {
	return strings.TrimSpace(resourceID) + "\x00" + strings.TrimSpace(name)
}

func (c *ResolvedContext) tool(resourceID, name string) Tool {
	if c == nil {
		return nil
	}
	return c.implementations[contextToolKey(resourceID, name)]
}

func (c *ResolvedContext) close() {
	if c == nil {
		return
	}
	for _, cleanup := range c.cleanups {
		if cleanup != nil {
			_ = cleanup()
		}
	}
	c.cleanups = nil
}

type ContextResourceReader interface {
	Get(context.Context, string) (ContextResource, error)
}

type ContextProvider interface {
	Kinds() []string
	AccessModes() []string
	Resolve(context.Context, ContextResource) ([]Tool, []ContextFact, error)
}

type ContextAuthorizer func(context.Context, ContextResource) error

type ContextResolver interface {
	Resolve(context.Context, ContextRequest) (ResolvedContext, error)
}

type ResourceContextResolver struct {
	Resources ContextResourceReader
	Providers []ContextProvider
	Authorize ContextAuthorizer
	Registry  *ToolRegistry
}

func NewResourceContextResolver(resources ContextResourceReader, registry *ToolRegistry, providers ...ContextProvider) ResourceContextResolver {
	return ResourceContextResolver{Resources: resources, Registry: registry, Providers: providers}
}

func (r ResourceContextResolver) Resolve(ctx context.Context, request ContextRequest) (ResolvedContext, error) {
	if r.Resources == nil {
		return ResolvedContext{}, fmt.Errorf("context resource reader is unavailable")
	}
	seen := make(map[string]struct{})
	resolved := ResolvedContext{Resources: []ContextResource{}, Tools: []ToolDefinition{}, Facts: []ContextFact{}, implementations: make(map[string]Tool)}
	completed := false
	defer func() {
		if !completed {
			resolved.close()
		}
	}()
	for _, rawID := range request.ResourceIDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			return ResolvedContext{}, fmt.Errorf("context resource id is required")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		resource, err := r.Resources.Get(ctx, id)
		if err != nil {
			return ResolvedContext{}, fmt.Errorf("resolve context resource %s: %w", id, err)
		}
		if strings.TrimSpace(resource.ID) == "" {
			resource.ID = id
		}
		if binding := request.ResourceBindings[id]; len(binding) > 0 {
			resource.Binding = cloneContextBinding(binding)
		}
		if r.Authorize != nil {
			if err := r.Authorize(ctx, resource); err != nil {
				return ResolvedContext{}, fmt.Errorf("authorize context resource %s: %w", id, err)
			}
		}
		if request.RestrictKinds {
			matches := false
			for _, kind := range request.AllowedKinds {
				if PersonaDomainMatches(kind, resource.Kind) {
					matches = true
					break
				}
			}
			if !matches {
				continue
			}
		}
		resolved.Resources = append(resolved.Resources, resource)
		if request.MetadataOnly {
			continue
		}
		provider, ok := providerFor(r.Providers, resource)
		if !ok {
			if strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
				return ResolvedContext{}, fmt.Errorf("no MCP context provider is available for agent resource %s", id)
			}
			continue
		}
		tools, facts, err := provider.Resolve(ctx, resource)
		if err != nil {
			return ResolvedContext{}, fmt.Errorf("resolve tools for context resource %s: %w", id, err)
		}
		for _, tool := range tools {
			definition := tool.Definition()
			if definition.ResourceID == "" {
				definition.ResourceID = resource.ID
			}
			definition.ResourceName = resource.Name
			if normalized, ok := tool.(ToolFunc); ok {
				normalized.Def = definition
				tool = normalized
			}
			resolved.implementations[contextToolKey(resource.ID, definition.Name)] = tool
			if closer, ok := tool.(interface{ Close() error }); ok {
				resolved.cleanups = append(resolved.cleanups, closer.Close)
			}
			resolved.Tools = append(resolved.Tools, definition)
		}
		resolved.Facts = append(resolved.Facts, facts...)
	}
	completed = true
	return resolved, nil
}

func cloneContextBinding(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func providerFor(providers []ContextProvider, resource ContextResource) (ContextProvider, bool) {
	for _, provider := range providers {
		for _, supported := range provider.Kinds() {
			if !strings.EqualFold(strings.TrimSpace(supported), strings.TrimSpace(resource.Kind)) {
				continue
			}
			if mode := strings.ToLower(strings.TrimSpace(resource.Subtype)); mode != "" {
				if !containsFold(provider.AccessModes(), mode) {
					continue
				}
			}
			return provider, true
		}
	}
	return nil, false
}

func containsFold(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), wanted) {
			return true
		}
	}
	return false
}
