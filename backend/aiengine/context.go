package aiengine

import (
	"context"
	"fmt"
	"strings"
)

type ContextResource struct {
	ID      string
	ScopeID string
	Kind    string
	Name    string
	Status  string
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
}

type ContextResourceReader interface {
	Get(context.Context, string) (ContextResource, error)
}

type ContextProvider interface {
	Kinds() []string
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
	resolved := ResolvedContext{Resources: []ContextResource{}, Tools: []ToolDefinition{}, Facts: []ContextFact{}}
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
		if r.Authorize != nil {
			if err := r.Authorize(ctx, resource); err != nil {
				return ResolvedContext{}, fmt.Errorf("authorize context resource %s: %w", id, err)
			}
		}
		if resource.Status != "" && resource.Status != "active" {
			return ResolvedContext{}, fmt.Errorf("context resource %s is not active", id)
		}
		resolved.Resources = append(resolved.Resources, resource)
		provider, ok := providerFor(r.Providers, resource.Kind)
		if !ok {
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
			if r.Registry != nil {
				if err := r.Registry.Upsert(resource.ID, tool); err != nil {
					return ResolvedContext{}, fmt.Errorf("register tool %s for context resource %s: %w", definition.Name, id, err)
				}
			}
			resolved.Tools = append(resolved.Tools, definition)
		}
		resolved.Facts = append(resolved.Facts, facts...)
	}
	return resolved, nil
}

func providerFor(providers []ContextProvider, kind string) (ContextProvider, bool) {
	for _, provider := range providers {
		for _, supported := range provider.Kinds() {
			if strings.EqualFold(strings.TrimSpace(supported), strings.TrimSpace(kind)) {
				return provider, true
			}
		}
	}
	return nil, false
}
