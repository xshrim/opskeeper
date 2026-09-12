package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opskeeper/backend/aiengine"
	"opskeeper/backend/mcpserver/docker/client"
	dockertool "opskeeper/backend/tool/docker"
)

// resolveDockerTools binds the protocol-neutral Docker implementation to one
// logical Direct resource. Connection fields are adapter-owned and overwrite
// any same-named values supplied by the model.
func (s *Service) resolveDockerTools(ctx context.Context, item aiengine.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (aiengine.ToolResult, error))) error {
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "agent") {
		return fmt.Errorf("Docker agent resources must use the MCP provider")
	}
	connection, err := s.dockerConnection(ctx, item)
	if err != nil {
		return err
	}

	add("docker_info", "Read Docker Engine information.", directDockerSchema(nil), func(runCtx context.Context, args map[string]any) (aiengine.ToolResult, error) {
		input, err := decodeDockerInput[dockertool.DockerInfoInput](args)
		if err != nil {
			return aiengine.ToolResult{}, err
		}
		input.ConnectionInput = connection
		return dockerOutput(dockertool.Info(runCtx, input))
	})
	add("docker_images", "List Docker images.", directDockerSchema(map[string]any{
		"all":     map[string]any{"type": "boolean", "description": "Include intermediate and dangling images."},
		"filters": map[string]any{"type": "string", "description": "Optional comma-separated Docker filters in key:value or key=value format."},
	}), func(runCtx context.Context, args map[string]any) (aiengine.ToolResult, error) {
		input, err := decodeDockerInput[dockertool.ListImagesInput](args)
		if err != nil {
			return aiengine.ToolResult{}, err
		}
		input.ConnectionInput = connection
		return dockerOutput(dockertool.Images(runCtx, input))
	})
	add("docker_containers", "List Docker containers.", directDockerSchema(map[string]any{
		"all":     map[string]any{"type": "boolean", "description": "Include stopped containers."},
		"limit":   map[string]any{"type": "integer", "description": "Maximum number of containers to return; 0 uses the Docker default and the maximum is 500."},
		"filters": map[string]any{"type": "string", "description": "Optional comma-separated Docker filters in key:value or key=value format."},
	}), func(runCtx context.Context, args map[string]any) (aiengine.ToolResult, error) {
		input, err := decodeDockerInput[dockertool.ListContainersInput](args)
		if err != nil {
			return aiengine.ToolResult{}, err
		}
		input.ConnectionInput = connection
		return dockerOutput(dockertool.Containers(runCtx, input))
	})
	add("docker_container_logs", "Read bounded, non-following logs from a Docker container.", directDockerSchema(map[string]any{
		"container_id":   map[string]any{"type": "string", "description": "Container ID or name; takes precedence over container_name."},
		"container_name": map[string]any{"type": "string", "description": "Container name when container_id is omitted."},
		"tail":           map[string]any{"type": "string", "description": "Number of lines requested from the end; defaults to 1000. Keyword filtering is applied first, then output is limited to the most recent 200 lines."},
		"since":          map[string]any{"type": "string", "description": "Only show logs since this timestamp or duration."},
		"until":          map[string]any{"type": "string", "description": "Only show logs until this timestamp."},
		"keyword":        map[string]any{"type": "string", "description": "Case-insensitive keyword filter. Separate terms with & to require all terms on the same line, or | to match any term; AND binds tighter than OR."},
		"timestamps":     map[string]any{"type": "boolean", "description": "Include timestamps in log lines."},
		"details":        map[string]any{"type": "boolean", "description": "Include extra Docker log attributes."},
	}), func(runCtx context.Context, args map[string]any) (aiengine.ToolResult, error) {
		input, err := decodeDockerInput[dockertool.ContainerLogsInput](args)
		if err != nil {
			return aiengine.ToolResult{}, err
		}
		input.ConnectionInput = connection
		return dockerOutput(dockertool.ContainerLogs(runCtx, input))
	})
	add("docker_container_file", "Read a bounded regular file from a Docker container.", directDockerSchema(map[string]any{
		"container_id":   map[string]any{"type": "string", "description": "Container ID or name; takes precedence over container_name."},
		"container_name": map[string]any{"type": "string", "description": "Container name when container_id is omitted."},
		"path":           map[string]any{"type": "string", "description": "Absolute path of a regular file inside the container."},
	}), func(runCtx context.Context, args map[string]any) (aiengine.ToolResult, error) {
		input, err := decodeDockerInput[dockertool.ContainerFileInput](args)
		if err != nil {
			return aiengine.ToolResult{}, err
		}
		input.ConnectionInput = connection
		return dockerOutput(dockertool.ContainerFile(runCtx, input))
	})
	add("docker_container_inspect", "Inspect a Docker container.", directDockerSchema(map[string]any{
		"container_id":   map[string]any{"type": "string", "description": "Container ID or name; takes precedence over container_name."},
		"container_name": map[string]any{"type": "string", "description": "Container name when container_id is omitted."},
	}), func(runCtx context.Context, args map[string]any) (aiengine.ToolResult, error) {
		input, err := decodeDockerInput[dockertool.ContainerInspectInput](args)
		if err != nil {
			return aiengine.ToolResult{}, err
		}
		input.ConnectionInput = connection
		return dockerOutput(dockertool.ContainerInspect(runCtx, input))
	})
	add("docker_container_stats", "Read one snapshot of Docker container statistics.", directDockerSchema(map[string]any{
		"container_id":   map[string]any{"type": "string", "description": "Container ID or name; takes precedence over container_name."},
		"container_name": map[string]any{"type": "string", "description": "Container name when container_id is omitted."},
	}), func(runCtx context.Context, args map[string]any) (aiengine.ToolResult, error) {
		input, err := decodeDockerInput[dockertool.ContainerStatsInput](args)
		if err != nil {
			return aiengine.ToolResult{}, err
		}
		input.ConnectionInput = connection
		return dockerOutput(dockertool.ContainerStats(runCtx, input))
	})
	return nil
}

func dockerSchema(extra map[string]any) json.RawMessage {
	encoded, _ := json.Marshal(dockertool.InputSchema(extra))
	return encoded
}

func directDockerSchema(extra map[string]any) json.RawMessage {
	schema := dockertool.InputSchema(extra)
	if properties, ok := schema["properties"].(map[string]any); ok {
		for _, name := range []string{"host", "timeout", "tls_ca", "tls_cert", "tls_key", "skip_tls_verify"} {
			delete(properties, name)
		}
	}
	encoded, _ := json.Marshal(schema)
	return encoded
}

func decodeDockerInput[T any](args map[string]any) (T, error) {
	var input T
	encoded, err := json.Marshal(args)
	if err != nil {
		return input, fmt.Errorf("encode Docker tool arguments: %w", err)
	}
	if err := json.Unmarshal(encoded, &input); err != nil {
		return input, fmt.Errorf("decode Docker tool arguments: %w", err)
	}
	return input, nil
}

func dockerOutput[T any](value T, err error) (aiengine.ToolResult, error) {
	if err != nil {
		return aiengine.ToolResult{}, err
	}
	return aiengine.ToolResult{Output: value, Untrusted: true}, nil
}

func (s *Service) dockerConnection(ctx context.Context, item aiengine.ContextResource) (client.ConnectionInput, error) {
	connection := client.ConnectionInput{}
	setDockerConnectionFromMap(&connection, item.Config)
	if item.CredentialID == nil || strings.TrimSpace(*item.CredentialID) == "" {
		return connection, nil
	}
	if s.credentials == nil {
		return client.ConnectionInput{}, connectorError(CategoryConfiguration, "read Docker credential", false, fmt.Errorf("credential service is unavailable"))
	}
	secret, err := s.credentials.RevealLinked(ctx, *item.CredentialID)
	if err != nil {
		return client.ConnectionInput{}, connectorError(CategoryConfiguration, "read Docker credential", false, err)
	}
	// Docker credentials may be supplied as a JSON object. Config values win,
	// so a credential cannot silently redirect an explicitly configured resource.
	var credentialValues map[string]any
	if json.Unmarshal(secret, &credentialValues) == nil {
		setDockerConnectionFromMapIfEmpty(&connection, credentialValues)
	}
	return connection, nil
}

func setDockerConnectionFromMap(connection *client.ConnectionInput, values map[string]any) {
	if connection == nil {
		return
	}
	connection.DockerHost = stringValue(values, "host")
	connection.TimeoutSeconds = values["timeout"]
	connection.DockerCA = stringValue(values, "tls_ca")
	connection.DockerCert = stringValue(values, "tls_cert")
	connection.DockerKey = stringValue(values, "tls_key")
	connection.DockerSkipVerify = boolValue(values, "skip_tls_verify")
}

func setDockerConnectionFromMapIfEmpty(connection *client.ConnectionInput, values map[string]any) {
	if connection == nil {
		return
	}
	if connection.DockerHost == "" {
		connection.DockerHost = stringValue(values, "host")
	}
	if client.TimeoutSeconds(connection.TimeoutSeconds) <= 0 {
		connection.TimeoutSeconds = values["timeout"]
	}
	if connection.DockerCA == "" {
		connection.DockerCA = stringValue(values, "tls_ca")
	}
	if connection.DockerCert == "" {
		connection.DockerCert = stringValue(values, "tls_cert")
	}
	if connection.DockerKey == "" {
		connection.DockerKey = stringValue(values, "tls_key")
	}
	if !connection.DockerSkipVerify {
		connection.DockerSkipVerify = boolValue(values, "skip_tls_verify")
	}
}

func stringValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func boolValue(values map[string]any, key string) bool {
	value, _ := values[key].(bool)
	return value
}
