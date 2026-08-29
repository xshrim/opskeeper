package aiengine

import (
	"context"
	"fmt"

	"opskeeper/backend/resource"
)

// ContextTooling is the standard T02 composition root for resource context
// resolution and tool execution. Callers should pass the same authorization
// context used by the API/Worker request to Resolve and Invoke.
type ContextTooling struct {
	Registry *ToolRegistry
	Resolver ResourceContextResolver
	Gateway  *PolicyGateway
}

func NewContextTooling(resources ContextResourceReader, providers ...ContextProvider) *ContextTooling {
	registry := NewToolRegistry()
	tooling := &ContextTooling{Registry: registry}
	tooling.Gateway = NewPolicyGateway(registry, func(ctx context.Context, call ToolCall, definition ToolDefinition) error {
		if definition.ResourceID != call.ResourceID {
			return fmt.Errorf("tool resource does not match call resource")
		}
		return AuthorizeResourceUse(ctx, ContextResource{ID: call.ResourceID, ScopeID: call.ScopeID})
	}, 0, 0, 0)
	tooling.Resolver = NewResourceContextResolver(resources, registry, providers...)
	tooling.Resolver.Authorize = AuthorizeResourceUse
	return tooling
}

// ResourceServiceReader adapts the existing resource service/store contract to
// the dependency-light ContextResourceReader used by AIEngine.
type ResourceServiceReader struct {
	Reader interface {
		Get(context.Context, string) (resource.Resource, error)
	}
}

func (r ResourceServiceReader) Get(ctx context.Context, id string) (ContextResource, error) {
	if r.Reader == nil {
		return ContextResource{}, fmt.Errorf("resource service is unavailable")
	}
	item, err := r.Reader.Get(ctx, id)
	if err != nil {
		return ContextResource{}, err
	}
	return ContextResource{ID: item.ID, ScopeID: item.ScopeID, Kind: item.Kind, Name: item.Name, Status: item.Status}, nil
}
