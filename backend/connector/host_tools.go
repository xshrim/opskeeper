package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opskeeper/backend/aiengine"
	host "opskeeper/backend/tool/host"
)

func (s *Service) resolveHostTools(ctx context.Context, item aiengine.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (aiengine.ToolResult, error))) error {
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "agent") {
		return fmt.Errorf("Host agent resources must use the MCP provider")
	}
	connection, err := s.hostConnection(ctx, item)
	if err != nil {
		return err
	}
	add("host_info", "Read Linux host identity and system information.", hostDirectSchema(nil), func(ctx context.Context, args map[string]any) (aiengine.ToolResult, error) {
		input, err := decodeHost[host.InfoInput](args)
		if err != nil {
			return aiengine.ToolResult{}, err
		}
		input.ConnectionInput = connection
		return hostOutput(host.Info(ctx, input))
	})
	add("host_metrics", "Read Linux host metrics as structured JSON.", hostDirectSchema(map[string]any{"sample_seconds": map[string]any{"type": "integer", "minimum": 0, "maximum": host.MaxSampleSeconds}}), func(ctx context.Context, args map[string]any) (aiengine.ToolResult, error) {
		input, err := decodeHost[host.MetricsInput](args)
		if err != nil {
			return aiengine.ToolResult{}, err
		}
		input.ConnectionInput = connection
		return hostOutput(host.Metrics(ctx, input))
	})
	add("host_processes", "Read information about selected Linux processes.", hostDirectSchema(map[string]any{"pid": map[string]any{"type": "integer", "minimum": 1}, "keyword": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": host.MaxProcessLimit}, "sample_seconds": map[string]any{"type": "integer", "minimum": 0, "maximum": host.MaxSampleSeconds}}), func(ctx context.Context, args map[string]any) (aiengine.ToolResult, error) {
		input, err := decodeHost[host.ProcessesInput](args)
		if err != nil {
			return aiengine.ToolResult{}, err
		}
		input.ConnectionInput = connection
		return hostOutput(host.Processes(ctx, input))
	})
	add("host_file_logs", "Read bounded logs from a Linux host file.", hostDirectSchema(map[string]any{"path": map[string]any{"type": "string"}, "tail": map[string]any{"type": "string"}, "since": map[string]any{"type": "string"}, "until": map[string]any{"type": "string"}, "keyword": map[string]any{"type": "string"}, "timestamps": map[string]any{"type": "boolean"}}), func(ctx context.Context, args map[string]any) (aiengine.ToolResult, error) {
		input, err := decodeHost[host.FileLogsInput](args)
		if err != nil {
			return aiengine.ToolResult{}, err
		}
		input.ConnectionInput = connection
		return hostOutput(host.FileLogs(ctx, input))
	})
	add("host_health", "Check Linux host health and availability.", hostDirectSchema(nil), func(ctx context.Context, args map[string]any) (aiengine.ToolResult, error) {
		input, err := decodeHost[host.ConnectionInput](args)
		if err != nil {
			return aiengine.ToolResult{}, err
		}
		input = connection
		return hostOutput(host.Health(ctx, input))
	})
	return nil
}

func hostDirectSchema(extra map[string]any) json.RawMessage {
	schema := host.InputSchema(extra)
	if properties, ok := schema["properties"].(map[string]any); ok {
		for _, key := range []string{"host", "port", "username", "auth_method", "password", "private_key", "passphrase", "known_hosts", "timeout_seconds"} {
			delete(properties, key)
		}
	}
	raw, _ := json.Marshal(schema)
	return raw
}
func decodeHost[T any](args map[string]any) (T, error) {
	var value T
	raw, err := json.Marshal(args)
	if err != nil {
		return value, fmt.Errorf("encode Host tool arguments: %w", err)
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return value, fmt.Errorf("decode Host tool arguments: %w", err)
	}
	return value, nil
}
func hostOutput[T any](value T, err error) (aiengine.ToolResult, error) {
	if err != nil {
		return aiengine.ToolResult{}, err
	}
	return aiengine.ToolResult{Output: value, Untrusted: true}, nil
}
func (s *Service) hostConnection(ctx context.Context, item aiengine.ContextResource) (host.ConnectionInput, error) {
	input := host.ConnectionInput{}
	setHostInput(&input, item.Config)
	if item.CredentialID == nil || strings.TrimSpace(*item.CredentialID) == "" {
		return input, nil
	}
	if s.credentials == nil {
		return host.ConnectionInput{}, connectorError(CategoryConfiguration, "read Host credential", false, fmt.Errorf("credential service is unavailable"))
	}
	secret, err := s.credentials.RevealLinked(ctx, *item.CredentialID)
	if err != nil {
		return host.ConnectionInput{}, connectorError(CategoryConfiguration, "read Host credential", false, err)
	}
	var values map[string]any
	if json.Unmarshal(secret, &values) == nil {
		setHostInputIfEmpty(&input, values)
	}
	return input, nil
}
