package mcp

import (
	"context"
	"fmt"

	"opskeeper/backend/aiengine"
)

// AIEngineProvider exposes an MCP resource as a context source. Discovery and
// calls still pass through Service, which enforces resource scope, HTTPS,
// allowlists, timeouts and response bounds.
func (s *Service) AIEngineProvider() aiengine.ContextProvider {
	return mcpContextProvider{service: s}
}

type mcpContextProvider struct{ service *Service }

func (mcpContextProvider) Kinds() []string { return []string{"MCPServer"} }

func (p mcpContextProvider) Resolve(ctx context.Context, resource aiengine.ContextResource) ([]aiengine.Tool, []aiengine.ContextFact, error) {
	if p.service == nil {
		return nil, nil, fmt.Errorf("MCP service is unavailable")
	}
	snapshot, err := p.service.Discover(ctx, resource.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("discover MCP tools: %w", err)
	}
	tools := make([]aiengine.Tool, 0, len(snapshot.Tools))
	for _, discovered := range snapshot.Tools {
		item := discovered
		tools = append(tools, aiengine.ToolFunc{
			Def: aiengine.ToolDefinition{
				Name: item.Name, Description: item.Description, InputSchema: item.InputSchema,
				Source: "mcp", ResourceID: resource.ID,
				// MCP metadata is untrusted and does not prove that a tool is read-only.
				ReadOnly: false,
			},
			Fn: func(runCtx context.Context, arguments map[string]any) (aiengine.ToolResult, error) {
				result, callErr := p.service.Call(runCtx, resource.ID, item.Name, arguments)
				if callErr != nil {
					return aiengine.ToolResult{}, callErr
				}
				return aiengine.ToolResult{Output: result, Untrusted: true}, nil
			},
		})
	}
	fact := aiengine.ContextFact{ResourceID: resource.ID, Kind: resource.Kind, Summary: map[string]any{"server_name": snapshot.ServerName, "server_version": snapshot.ServerVersion, "tool_count": len(snapshot.Tools)}, Untrusted: true}
	return tools, []aiengine.ContextFact{fact}, nil
}
