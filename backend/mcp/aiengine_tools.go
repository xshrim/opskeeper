package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opskeeper/backend/aiengine"
)

// AIEngineProvider exposes MCPServer resources and Agent logical resources as
// context sources. Discovery and calls still pass through Service, which
// enforces resource scope, HTTPS, allowlists, timeouts and response bounds.
func (s *Service) AIEngineProvider() aiengine.ContextProvider {
	return mcpContextProvider{service: s}
}

type mcpContextProvider struct{ service *Service }

func (mcpContextProvider) Kinds() []string {
	return []string{"MCPServer", "Host", "Docker", "Kubernetes", "Nacos", "Repository", "Redis", "PostgreSQL", "Kafka", "RabbitMQ", "Elasticsearch", "OceanBase", "Oracle", "MySQL", "TongRDS", "Prometheus", "Loki"}
}

func (mcpContextProvider) AccessModes() []string { return []string{"agent"} }

func (p mcpContextProvider) Resolve(ctx context.Context, resource aiengine.ContextResource) ([]aiengine.Tool, []aiengine.ContextFact, error) {
	if p.service == nil {
		return nil, nil, fmt.Errorf("MCP service is unavailable")
	}
	transportResourceID := resource.ID
	if strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
		if resource.AgentRef == nil || strings.TrimSpace(*resource.AgentRef) == "" {
			return nil, nil, fmt.Errorf("agent resource requires an MCPServer association")
		}
		transportResourceID = strings.TrimSpace(*resource.AgentRef)
	}
	snapshot, err := p.service.Discover(ctx, transportResourceID)
	if err != nil {
		return nil, nil, fmt.Errorf("discover MCP tools: %w", err)
	}
	tools := make([]aiengine.Tool, 0, len(snapshot.Tools))
	for _, discovered := range snapshot.Tools {
		item := discovered
		inputSchema := item.InputSchema
		if strings.EqualFold(strings.TrimSpace(resource.Kind), "Host") && strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			inputSchema = hostAgentSchema(inputSchema)
		} else if strings.EqualFold(strings.TrimSpace(resource.Kind), "PostgreSQL") && strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			inputSchema = postgreSQLAgentSchema(inputSchema)
		} else if strings.EqualFold(strings.TrimSpace(resource.Kind), "Redis") && strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			inputSchema = redisAgentSchema(inputSchema)
		} else if strings.EqualFold(strings.TrimSpace(resource.Kind), "Nacos") && strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			inputSchema = nacosAgentSchema(inputSchema)
		} else if strings.EqualFold(strings.TrimSpace(resource.Kind), "Repository") && strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			inputSchema = repositoryAgentSchema(inputSchema)
		}
		tools = append(tools, aiengine.ToolFunc{
			Def: aiengine.ToolDefinition{
				Name: item.Name, Description: item.Description, InputSchema: inputSchema,
				Source: "mcp", ResourceID: resource.ID,
				// MCP metadata is untrusted and does not prove that a tool is read-only.
				ReadOnly: false,
			},
			Fn: func(runCtx context.Context, arguments map[string]any) (aiengine.ToolResult, error) {
				callArguments, err := p.agentArguments(runCtx, resource, arguments)
				if err != nil {
					return aiengine.ToolResult{}, err
				}
				result, callErr := p.service.Call(runCtx, transportResourceID, item.Name, callArguments)
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

// dockerAgentArguments injects a Docker Agent's resource-owned connection
// settings into the forwarded MCP call. The model cannot override configured
// values, and TLS material never enters the model-facing tool schema.
func (p mcpContextProvider) dockerAgentArguments(ctx context.Context, contextResource aiengine.ContextResource, arguments map[string]any) (map[string]any, error) {
	return p.agentArguments(ctx, contextResource, arguments)
}

func (p mcpContextProvider) agentArguments(ctx context.Context, contextResource aiengine.ContextResource, arguments map[string]any) (map[string]any, error) {
	kind := strings.TrimSpace(contextResource.Kind)
	if (!strings.EqualFold(kind, "Docker") && !strings.EqualFold(kind, "Kubernetes") && !strings.EqualFold(kind, "Host") && !strings.EqualFold(kind, "PostgreSQL") && !strings.EqualFold(kind, "Redis") && !strings.EqualFold(kind, "Nacos") && !strings.EqualFold(kind, "Repository")) || !strings.EqualFold(strings.TrimSpace(contextResource.Subtype), "agent") {
		return arguments, nil
	}
	if len(contextResource.Config) == 0 && (contextResource.CredentialID == nil || strings.TrimSpace(*contextResource.CredentialID) == "") {
		return arguments, nil
	}
	merged := make(map[string]any, len(arguments)+12)
	for key, value := range arguments {
		merged[key] = value
	}
	for key, value := range contextResource.Config {
		if value != nil {
			merged[key] = value
		}
	}
	setString := func(key string) {
		if value, ok := contextResource.Config[key].(string); ok && strings.TrimSpace(value) != "" {
			merged[key] = strings.TrimSpace(value)
		}
	}
	if strings.EqualFold(kind, "Kubernetes") {
		for _, key := range []string{"kubeconfig", "connection_mode", "context", "profile", "server", "ca", "token", "client_cert", "client_key"} {
			setString := func() {
				if value, ok := contextResource.Config[key].(string); ok && strings.TrimSpace(value) != "" {
					merged[key] = strings.TrimSpace(value)
				}
			}
			setString()
		}
		if value, ok := contextResource.Config["skip_tls_verify"].(bool); ok {
			merged["skip_tls_verify"] = value
		}
	} else if strings.EqualFold(kind, "Host") {
		for _, key := range []string{"host", "port", "username", "auth_method", "password", "private_key", "passphrase", "known_hosts", "timeout_seconds"} {
			setString(key)
		}
	} else if strings.EqualFold(kind, "PostgreSQL") {
		for _, key := range []string{"host", "port", "database", "username", "password", "timeout_seconds"} {
			setString(key)
		}
	} else if strings.EqualFold(kind, "Redis") {
		for _, key := range []string{"host", "port", "database", "username", "password", "timeout_seconds"} {
			setString(key)
		}
	} else if strings.EqualFold(kind, "Nacos") {
		for _, key := range []string{"host", "port", "scheme", "context_path", "username", "password", "access_token", "timeout_seconds"} {
			setString(key)
		}
	} else if strings.EqualFold(kind, "Repository") {
		for _, key := range []string{"path", "url", "default_branch", "timeout_seconds"} {
			setString(key)
		}
	} else {
		setString("host")
	}
	for _, key := range []string{"tls_ca", "tls_cert", "tls_key"} {
		// These values are normally credential-owned; config support also keeps
		// compatibility with resources that store non-secret paths there.
		setString(key)
	}
	if value, ok := contextResource.Config["timeout"]; ok {
		merged["timeout"] = value
	}
	if value, ok := contextResource.Config["skip_tls_verify"].(bool); ok {
		merged["skip_tls_verify"] = value
	}
	if contextResource.CredentialID == nil || strings.TrimSpace(*contextResource.CredentialID) == "" || p.service.credentials == nil {
		return merged, nil
	}
	secret, err := p.service.credentials.RevealLinked(ctx, *contextResource.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("read %s Agent credential: %w", kind, err)
	}
	var values map[string]any
	if err := json.Unmarshal(secret, &values); err != nil {
		return merged, nil
	}
	connectionMode, _ := contextResource.Config["connection_mode"].(string)
	if strings.EqualFold(kind, "Kubernetes") {
		if strings.EqualFold(strings.TrimSpace(connectionMode), "endpoint") {
			delete(merged, "kubeconfig")
		} else if _, configured := contextResource.Config["kubeconfig"]; !configured {
			if raw, ok := values["kubeconfig"].(string); ok && strings.TrimSpace(raw) != "" {
				merged["kubeconfig"] = strings.TrimSpace(raw)
			}
		}
	}
	keys := []string{"tls_ca", "tls_cert", "tls_key", "host"}
	if strings.EqualFold(kind, "Kubernetes") {
		keys = []string{"kubeconfig", "connection_mode", "context", "profile", "server", "ca", "token", "client_cert", "client_key"}
	} else if strings.EqualFold(kind, "Host") {
		keys = []string{"host", "port", "username", "auth_method", "password", "private_key", "passphrase", "known_hosts", "timeout_seconds"}
	} else if strings.EqualFold(kind, "PostgreSQL") {
		keys = []string{"host", "port", "database", "username", "password", "timeout_seconds"}
	} else if strings.EqualFold(kind, "Redis") {
		keys = []string{"host", "port", "database", "username", "password", "timeout_seconds"}
	} else if strings.EqualFold(kind, "Nacos") {
		keys = []string{"host", "port", "scheme", "context_path", "username", "password", "access_token", "timeout_seconds"}
	} else if strings.EqualFold(kind, "Repository") {
		keys = []string{"path", "url", "default_branch", "timeout_seconds"}
	}
	for _, key := range keys {
		if strings.EqualFold(strings.TrimSpace(connectionMode), "endpoint") && key == "kubeconfig" {
			continue
		}
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			// Explicit resource config wins over credential values.
			if _, configured := contextResource.Config[key]; !configured {
				merged[key] = strings.TrimSpace(value)
			}
		}
	}
	if _, configured := contextResource.Config["timeout"]; !configured {
		if value, ok := values["timeout"]; ok {
			merged["timeout"] = value
		}
	}
	if _, configured := contextResource.Config["skip_tls_verify"]; !configured {
		if value, ok := values["skip_tls_verify"].(bool); ok {
			merged["skip_tls_verify"] = value
		}
	}
	return merged, nil
}

func hostAgentSchema(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var schema map[string]any
	if json.Unmarshal(raw, &schema) != nil {
		return raw
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		for _, key := range []string{"host", "port", "username", "auth_method", "password", "private_key", "passphrase", "known_hosts", "timeout_seconds"} {
			delete(properties, key)
		}
	}
	if required, ok := schema["required"].([]any); ok {
		filtered := make([]any, 0, len(required))
		for _, value := range required {
			if key, ok := value.(string); ok && !isHostConnectionKey(key) {
				filtered = append(filtered, key)
			}
		}
		if len(filtered) == 0 {
			delete(schema, "required")
		} else {
			schema["required"] = filtered
		}
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return raw
	}
	return encoded
}

func postgreSQLAgentSchema(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var schema map[string]any
	if json.Unmarshal(raw, &schema) != nil {
		return raw
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		for _, key := range []string{"host", "port", "database", "username", "password", "timeout_seconds"} {
			delete(properties, key)
		}
	}
	delete(schema, "required")
	encoded, _ := json.Marshal(schema)
	return encoded
}

func repositoryAgentSchema(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var schema map[string]any
	if json.Unmarshal(raw, &schema) != nil {
		return raw
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		for _, key := range []string{"path", "url", "default_branch", "timeout_seconds"} {
			delete(properties, key)
		}
	}
	delete(schema, "required")
	encoded, _ := json.Marshal(schema)
	return encoded
}

func redisAgentSchema(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var schema map[string]any
	if json.Unmarshal(raw, &schema) != nil {
		return raw
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		for _, key := range []string{"host", "port", "database", "username", "password", "timeout_seconds"} {
			delete(properties, key)
		}
	}
	delete(schema, "required")
	encoded, _ := json.Marshal(schema)
	return encoded
}

func nacosAgentSchema(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var schema map[string]any
	if json.Unmarshal(raw, &schema) != nil {
		return raw
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		for _, key := range []string{"host", "port", "scheme", "context_path", "username", "password", "access_token", "timeout_seconds"} {
			delete(properties, key)
		}
	}
	delete(schema, "required")
	encoded, _ := json.Marshal(schema)
	return encoded
}

func isHostConnectionKey(key string) bool {
	switch key {
	case "host", "port", "username", "auth_method", "password", "private_key", "passphrase", "known_hosts", "timeout_seconds":
		return true
	default:
		return false
	}
}
