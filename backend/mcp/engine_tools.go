package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"opskeeper/backend/engine"
	"opskeeper/backend/resource"
	elasticsearch "opskeeper/backend/tool/elasticsearch"
	kafka "opskeeper/backend/tool/kafka"
	kubernetes "opskeeper/backend/tool/kubernetes"
	minio "opskeeper/backend/tool/minio"
	mysql "opskeeper/backend/tool/mysql"
	nacos "opskeeper/backend/tool/nacos"
	oceanbase "opskeeper/backend/tool/oceanbase"
	oracle "opskeeper/backend/tool/oracle"
	postgresql "opskeeper/backend/tool/postgresql"
	rabbitmq "opskeeper/backend/tool/rabbitmq"
	redis "opskeeper/backend/tool/redis"
	repository "opskeeper/backend/tool/repository"
)

// EngineProvider exposes MCPServer resources and Agent logical resources as
// context sources. Discovery and calls still pass through Service, which
// enforces resource scope, HTTPS, allowlists, timeouts and response bounds.
func (s *Service) EngineProvider() engine.ContextProvider {
	return mcpContextProvider{service: s}
}

type mcpContextProvider struct{ service *Service }

func (mcpContextProvider) Kinds() []string {
	return []string{"MCPServer", "Host", "Docker", "Kubernetes", "Nacos", "Repository", "Redis", "PostgreSQL", "Kafka", "RabbitMQ", "Elasticsearch", "MinIO", "OceanBase", "Oracle", "MySQL", "TongRDS", "Prometheus", "Loki"}
}

func (mcpContextProvider) AccessModes() []string {
	return []string{"agent", "streamhttp", "streamable_http", "sse"}
}

func (p mcpContextProvider) Resolve(ctx context.Context, resource engine.ContextResource) ([]engine.Tool, []engine.ContextFact, error) {
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
	allowedAgentTools := agentToolNames(resource.Kind)
	tools := make([]engine.Tool, 0, len(snapshot.Tools))
	for _, discovered := range snapshot.Tools {
		item := discovered
		if strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			if _, allowed := allowedAgentTools[item.Name]; !allowed {
				continue
			}
		}
		inputSchema := item.InputSchema
		if strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") && strings.EqualFold(strings.TrimSpace(resource.Kind), "Docker") {
			inputSchema = dockerAgentSchema(inputSchema)
		} else if strings.EqualFold(strings.TrimSpace(resource.Kind), "Kubernetes") && strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			inputSchema = kubernetesAgentSchema(inputSchema)
		} else if strings.EqualFold(strings.TrimSpace(resource.Kind), "Host") && strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			inputSchema = hostAgentSchema(inputSchema)
		} else if strings.EqualFold(strings.TrimSpace(resource.Kind), "PostgreSQL") && strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			inputSchema = postgreSQLAgentSchema(inputSchema)
		} else if strings.EqualFold(strings.TrimSpace(resource.Kind), "MySQL") && strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			inputSchema = mysqlAgentSchema(inputSchema)
		} else if strings.EqualFold(strings.TrimSpace(resource.Kind), "Oracle") && strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			inputSchema = oracleAgentSchema(inputSchema)
		} else if strings.EqualFold(strings.TrimSpace(resource.Kind), "OceanBase") && strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			inputSchema = oceanBaseAgentSchema(inputSchema)
		} else if strings.EqualFold(strings.TrimSpace(resource.Kind), "RabbitMQ") && strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			inputSchema = rabbitMQAgentSchema(inputSchema)
		} else if strings.EqualFold(strings.TrimSpace(resource.Kind), "MinIO") && strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			inputSchema = minIOAgentSchema(inputSchema)
		} else if strings.EqualFold(strings.TrimSpace(resource.Kind), "Redis") && strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			inputSchema = redisAgentSchema(inputSchema)
		} else if strings.EqualFold(strings.TrimSpace(resource.Kind), "Nacos") && strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			inputSchema = nacosAgentSchema(inputSchema)
		} else if strings.EqualFold(strings.TrimSpace(resource.Kind), "Repository") && strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			inputSchema = repositoryAgentSchema(inputSchema)
		} else if strings.EqualFold(strings.TrimSpace(resource.Kind), "Kafka") && strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			inputSchema = kafkaAgentSchema(inputSchema)
		} else if strings.EqualFold(strings.TrimSpace(resource.Kind), "Elasticsearch") && strings.EqualFold(strings.TrimSpace(resource.Subtype), "agent") {
			inputSchema = elasticsearchAgentSchema(inputSchema)
		}
		tools = append(tools, engine.ToolFunc{
			Def: engine.ToolDefinition{
				Name: item.Name, Description: item.Description, InputSchema: inputSchema,
				Source: "mcp", ResourceID: resource.ID,
				// MCP metadata is untrusted and does not prove that a tool is read-only.
				ReadOnly: false,
			},
			Fn: func(runCtx context.Context, arguments map[string]any) (engine.ToolResult, error) {
				callArguments, err := p.agentArguments(runCtx, resource, arguments)
				if err != nil {
					return engine.ToolResult{}, err
				}
				result, callErr := p.service.Call(runCtx, transportResourceID, item.Name, callArguments)
				if callErr != nil {
					return engine.ToolResult{}, callErr
				}
				return engine.ToolResult{Output: result, Untrusted: true}, nil
			},
		})
	}
	fact := engine.ContextFact{ResourceID: resource.ID, Kind: resource.Kind, Summary: map[string]any{"server_name": snapshot.ServerName, "server_version": snapshot.ServerVersion, "tool_count": len(tools)}, Untrusted: true}
	return tools, []engine.ContextFact{fact}, nil
}

func agentToolNames(kind string) map[string]struct{} {
	names := make([]string, 0)
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "host":
		names = []string{"host_info", "host_metrics", "host_processes", "host_file_logs", "host_health"}
	case "docker":
		names = []string{"docker_info", "docker_images", "docker_containers", "docker_container_logs", "docker_container_file", "docker_container_inspect", "docker_container_stats"}
	case "kubernetes":
		for _, tool := range kubernetes.AvailableTools() {
			names = append(names, tool.Name)
		}
	case "nacos":
		for _, tool := range nacos.ListTools() {
			names = append(names, tool.Name)
		}
	case "repository":
		for _, tool := range repository.ListTools() {
			names = append(names, tool.Name)
		}
	case "redis":
		for _, tool := range redis.ListTools() {
			names = append(names, tool.Name)
		}
	case "postgresql":
		for _, tool := range postgresql.ListTools() {
			names = append(names, tool.Name)
		}
	case "mysql":
		for _, tool := range mysql.ListTools() {
			names = append(names, tool.Name)
		}
	case "oracle":
		for _, tool := range oracle.ListTools() {
			names = append(names, tool.Name)
		}
	case "oceanbase":
		for _, tool := range oceanbase.ListTools() {
			names = append(names, tool.Name)
		}
	case "kafka":
		for _, tool := range kafka.ListTools() {
			names = append(names, tool.Name)
		}
	case "rabbitmq":
		for _, tool := range rabbitmq.ListTools() {
			names = append(names, tool.Name)
		}
	case "minio":
		for _, tool := range minio.ListTools() {
			names = append(names, tool.Name)
		}
	case "elasticsearch":
		for _, tool := range elasticsearch.ListTools() {
			names = append(names, tool.Name)
		}
	case "prometheus":
		names = []string{"connector.query_metrics", "connector.get_alerts"}
	case "loki":
		names = []string{"connector.query_logs"}
	default:
		return nil
	}
	allowed := make(map[string]struct{}, len(names))
	for _, name := range names {
		allowed[name] = struct{}{}
	}
	return allowed
}

func kubernetesAgentSchema(raw json.RawMessage) json.RawMessage {
	encoded := hideConnectionFields(raw, "kubeconfig", "connection_mode", "context", "profile", "server", "ca", "token", "client_cert", "client_key", "skip_tls_verify")
	var schema map[string]any
	if json.Unmarshal(encoded, &schema) != nil {
		return raw
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		properties["context"] = map[string]any{"type": "string", "description": "Kubeconfig context for this call; defaults to the resource or MCP Server context."}
	}
	result, err := json.Marshal(schema)
	if err != nil {
		return raw
	}
	return result
}

func hideConnectionFields(raw json.RawMessage, keys ...string) json.RawMessage {
	if len(raw) == 0 || len(keys) == 0 {
		return raw
	}
	var schema map[string]any
	if json.Unmarshal(raw, &schema) != nil {
		return raw
	}
	hidden := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		hidden[key] = struct{}{}
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		for key := range hidden {
			delete(properties, key)
		}
	}
	if required, ok := schema["required"].([]any); ok {
		kept := make([]any, 0, len(required))
		for _, value := range required {
			key, ok := value.(string)
			if !ok {
				kept = append(kept, value)
				continue
			}
			if _, remove := hidden[key]; !remove {
				kept = append(kept, value)
			}
		}
		if len(kept) == 0 {
			delete(schema, "required")
		} else {
			schema["required"] = kept
		}
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return raw
	}
	return encoded
}

// dockerAgentArguments injects a Docker Agent's resource-owned connection
// settings into the forwarded MCP call. The model cannot override configured
// values, and TLS material never enters the model-facing tool schema.
func (p mcpContextProvider) dockerAgentArguments(ctx context.Context, contextResource engine.ContextResource, arguments map[string]any) (map[string]any, error) {
	return p.agentArguments(ctx, contextResource, arguments)
}

func (p mcpContextProvider) agentArguments(ctx context.Context, contextResource engine.ContextResource, arguments map[string]any) (map[string]any, error) {
	kind := strings.TrimSpace(contextResource.Kind)
	if (!strings.EqualFold(kind, "Docker") && !strings.EqualFold(kind, "Kubernetes") && !strings.EqualFold(kind, "Host") && !strings.EqualFold(kind, "PostgreSQL") && !strings.EqualFold(kind, "MySQL") && !strings.EqualFold(kind, "Oracle") && !strings.EqualFold(kind, "OceanBase") && !strings.EqualFold(kind, "Redis") && !strings.EqualFold(kind, "Nacos") && !strings.EqualFold(kind, "Repository") && !strings.EqualFold(kind, "Kafka") && !strings.EqualFold(kind, "RabbitMQ") && !strings.EqualFold(kind, "Elasticsearch") && !strings.EqualFold(kind, "MinIO")) || !strings.EqualFold(strings.TrimSpace(contextResource.Subtype), "agent") {
		return arguments, nil
	}
	requestedKubernetesContext, _ := arguments["context"].(string)
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
		if strings.TrimSpace(requestedKubernetesContext) != "" {
			merged["context"] = strings.TrimSpace(requestedKubernetesContext)
		}
	} else if strings.EqualFold(kind, "Host") {
		for _, key := range []string{"host", "port", "username", "auth_method", "password", "private_key", "passphrase", "known_hosts", "timeout_seconds"} {
			setString(key)
		}
	} else if strings.EqualFold(kind, "PostgreSQL") {
		for _, key := range []string{"host", "port", "database", "username", "password", "timeout_seconds"} {
			setString(key)
		}
	} else if strings.EqualFold(kind, "MySQL") {
		for _, key := range []string{"host", "port", "database", "username", "password", "timeout_seconds"} {
			setString(key)
		}
	} else if strings.EqualFold(kind, "Oracle") {
		for _, key := range []string{"host", "port", "service_name", "sid", "username", "password", "timeout_seconds", "tls"} {
			if value, ok := contextResource.Config[key]; ok && value != nil {
				merged[key] = value
			}
		}
	} else if strings.EqualFold(kind, "OceanBase") {
		for _, key := range []string{"host", "port", "mode", "database", "service_name", "username", "password", "timeout_seconds"} {
			if value, ok := contextResource.Config[key]; ok && value != nil {
				merged[key] = value
			}
		}
	} else if strings.EqualFold(kind, "RabbitMQ") {
		for _, key := range []string{"url", "timeout_seconds", "tls_insecure"} {
			if value, ok := contextResource.Config[key]; ok && value != nil {
				merged[key] = value
			}
		}
	} else if strings.EqualFold(kind, "MinIO") {
		for _, key := range []string{"endpoint", "region", "secure", "timeout_seconds"} {
			if value, ok := contextResource.Config[key]; ok && value != nil {
				merged[key] = value
			}
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
		for _, key := range []string{"path", "url", "default_branch", "timeout_seconds", "storage_backend", "storage_key", "s3_endpoint", "s3_bucket", "s3_prefix"} {
			setString(key)
		}
	} else if strings.EqualFold(kind, "Kafka") {
		for _, key := range []string{"brokers", "tls", "tls_server_name", "timeout_seconds"} {
			if value, ok := contextResource.Config[key]; ok && value != nil {
				merged[key] = value
			}
		}
	} else if strings.EqualFold(kind, "Elasticsearch") {
		for _, key := range []string{"url", "tls_insecure", "timeout_seconds"} {
			if value, ok := contextResource.Config[key]; ok && value != nil {
				merged[key] = value
			}
		}
	} else {
		setString("host")
	}
	for _, key := range []string{"tls_ca", "tls_cert", "tls_key"} {
		// These values may be resource secrets; config support also permits
		// non-secret paths when a connector explicitly needs them.
		setString(key)
	}
	if value, ok := contextResource.Config["timeout"]; ok {
		merged["timeout"] = value
	}
	if value, ok := contextResource.Config["skip_tls_verify"].(bool); ok {
		merged["skip_tls_verify"] = value
	}
	reader, ok := p.service.resources.(interface {
		RevealSecret(context.Context, string) ([]byte, error)
	})
	if !ok {
		return merged, nil
	}
	secret, err := reader.RevealSecret(ctx, contextResource.ID)
	if err != nil {
		if errors.Is(err, resource.ErrNotFound) {
			return merged, nil
		}
		return nil, fmt.Errorf("read %s Agent resource secret: %w", kind, err)
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
	} else if strings.EqualFold(kind, "MySQL") {
		keys = []string{"host", "port", "database", "username", "password", "timeout_seconds"}
	} else if strings.EqualFold(kind, "Oracle") {
		keys = []string{"host", "port", "service_name", "sid", "username", "password", "timeout_seconds"}
	} else if strings.EqualFold(kind, "OceanBase") {
		keys = []string{"host", "port", "mode", "database", "service_name", "username", "password", "timeout_seconds"}
	} else if strings.EqualFold(kind, "RabbitMQ") {
		keys = []string{"url", "username", "password", "timeout_seconds", "tls_insecure"}
	} else if strings.EqualFold(kind, "MinIO") {
		keys = []string{"endpoint", "access_key", "secret_key", "session_token", "region", "timeout_seconds", "secure"}
	} else if strings.EqualFold(kind, "Redis") {
		keys = []string{"host", "port", "database", "username", "password", "timeout_seconds"}
	} else if strings.EqualFold(kind, "Nacos") {
		keys = []string{"host", "port", "scheme", "context_path", "username", "password", "access_token", "timeout_seconds"}
	} else if strings.EqualFold(kind, "Repository") {
		keys = []string{"path", "url", "default_branch", "timeout_seconds"}
	} else if strings.EqualFold(kind, "Kafka") {
		keys = []string{"brokers", "username", "password", "tls_server_name", "timeout_seconds"}
	} else if strings.EqualFold(kind, "Elasticsearch") {
		keys = []string{"url", "username", "password", "tls_insecure", "timeout_seconds"}
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

func dockerAgentSchema(raw json.RawMessage) json.RawMessage {
	return hideConnectionFields(raw, "host", "timeout", "tls_ca", "tls_cert", "tls_key", "skip_tls_verify")
}

func hostAgentSchema(raw json.RawMessage) json.RawMessage {
	return hideConnectionFields(raw, "host", "port", "username", "auth_method", "password", "private_key", "passphrase", "known_hosts", "timeout_seconds")
}

func postgreSQLAgentSchema(raw json.RawMessage) json.RawMessage {
	return hideConnectionFields(raw, "host", "port", "database", "username", "password", "timeout_seconds")
}

func repositoryAgentSchema(raw json.RawMessage) json.RawMessage {
	return hideConnectionFields(raw, "path", "url", "default_branch", "timeout_seconds", "storage_backend", "storage_key", "s3_endpoint", "s3_bucket", "s3_prefix")
}

func redisAgentSchema(raw json.RawMessage) json.RawMessage {
	return hideConnectionFields(raw, "host", "port", "database", "username", "password", "timeout_seconds")
}

func mysqlAgentSchema(raw json.RawMessage) json.RawMessage {
	return postgreSQLAgentSchema(raw)
}

func oracleAgentSchema(raw json.RawMessage) json.RawMessage {
	return hideConnectionFields(raw, "host", "port", "service_name", "sid", "username", "password", "timeout_seconds", "tls")
}
func oceanBaseAgentSchema(raw json.RawMessage) json.RawMessage {
	return hideConnectionFields(raw, "host", "port", "mode", "database", "service_name", "username", "password", "timeout_seconds")
}
func rabbitMQAgentSchema(raw json.RawMessage) json.RawMessage {
	return hideConnectionFields(raw, "url", "username", "password", "timeout_seconds", "tls_insecure")
}
func minIOAgentSchema(raw json.RawMessage) json.RawMessage {
	return hideConnectionFields(raw, "endpoint", "access_key", "secret_key", "session_token", "region", "secure", "timeout_seconds")
}
func nacosAgentSchema(raw json.RawMessage) json.RawMessage {
	return hideConnectionFields(raw, "host", "port", "scheme", "context_path", "username", "password", "access_token", "timeout_seconds")
}

func kafkaAgentSchema(raw json.RawMessage) json.RawMessage {
	return hideConnectionFields(raw, "brokers", "username", "password", "tls", "tls_server_name", "timeout_seconds")
}

func elasticsearchAgentSchema(raw json.RawMessage) json.RawMessage {
	return hideConnectionFields(raw, "url", "username", "password", "tls_insecure", "timeout_seconds")
}
