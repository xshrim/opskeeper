package connector

import (
	"context"
	"encoding/json"
	"fmt"

	"opskeeper/backend/aiengine"
	"opskeeper/backend/resource"
)

func (s *Service) invokeApplicationSource(ctx context.Context, source resource.Resource, tool string, arguments map[string]any) (aiengine.ToolResult, error) {
	if s == nil || s.applicationTools == nil {
		return aiengine.ToolResult{}, fmt.Errorf("Application source tool invoker is unavailable")
	}
	return s.applicationTools.InvokeFixed(ctx, aiengine.ContextResource{
		ID: source.ID, ScopeID: source.ScopeID, Kind: source.Kind, Name: source.Name,
		Status: source.Status, Subtype: source.Subtype, AgentRef: source.AgentRef,
		CredentialID: source.CredentialID, Config: source.Config,
	}, tool, arguments)
}

func decodeApplicationTool[T any](result aiengine.ToolResult) (T, error) {
	var output T
	raw, err := json.Marshal(result.Output)
	if err != nil {
		return output, fmt.Errorf("encode Application source tool output: %w", err)
	}
	if err := json.Unmarshal(raw, &output); err != nil {
		return output, fmt.Errorf("decode Application source tool output: %w", err)
	}
	return output, nil
}
