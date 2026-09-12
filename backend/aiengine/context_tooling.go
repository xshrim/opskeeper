package aiengine

import (
	"context"
	"fmt"
	"strings"
	"time"

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

// FixedResourceToolInvoker invokes one named tool for a resource through the
// provider selected by that resource's own access mode. Callers provide only
// a fixed, server-owned operation and arguments; they never choose transport.
type FixedResourceToolInvoker interface {
	InvokeFixed(context.Context, ContextResource, string, map[string]any) (ToolResult, error)
}

type ResourceToolInvoker struct{ Providers []ContextProvider }

func NewResourceToolInvoker(providers ...ContextProvider) ResourceToolInvoker {
	return ResourceToolInvoker{Providers: providers}
}

func (i ResourceToolInvoker) InvokeFixed(ctx context.Context, resource ContextResource, name string, arguments map[string]any) (ToolResult, error) {
	provider, ok := providerFor(i.Providers, resource)
	if !ok {
		return ToolResult{}, fmt.Errorf("no context provider is available for resource %s", resource.ID)
	}
	tools, _, err := provider.Resolve(ctx, resource)
	if err != nil {
		return ToolResult{}, err
	}
	for _, tool := range tools {
		if sameFixedToolName(tool.Definition().Name, name) {
			return tool.Invoke(ctx, arguments)
		}
	}
	return ToolResult{}, fmt.Errorf("resource %s does not provide required tool %q", resource.ID, name)
}

func sameFixedToolName(got, wanted string) bool {
	got = strings.TrimSpace(got)
	wanted = strings.TrimSpace(wanted)
	if strings.EqualFold(got, wanted) {
		return true
	}
	return strings.EqualFold(strings.TrimPrefix(got, "connector."), wanted) || strings.EqualFold(got, strings.TrimPrefix(wanted, "connector."))
}

func NewContextTooling(resources ContextResourceReader, providers ...ContextProvider) *ContextTooling {
	registry := NewToolRegistry()
	tooling := &ContextTooling{Registry: registry}
	// Host metrics can spend up to ten seconds sampling between two remote
	// snapshots, plus several SSH reads per pass.
	tooling.Gateway = NewPolicyGateway(registry, func(ctx context.Context, call ToolCall, definition ToolDefinition) error {
		if definition.ResourceID != call.ResourceID {
			return fmt.Errorf("tool resource does not match call resource")
		}
		return AuthorizeResourceUse(ctx, ContextResource{ID: call.ResourceID, ScopeID: call.ScopeID})
	}, 60*time.Second, 0, 0)
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
	return ContextResource{ID: item.ID, ScopeID: item.ScopeID, Kind: item.Kind, Name: item.Name, Status: item.Status, Subtype: item.Subtype, AgentRef: item.AgentRef, CredentialID: item.CredentialID, Config: item.Config}, nil
}
