package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opskeeper/backend/aiengine"
	rt "opskeeper/backend/tool/redis"
)

func (s *Service) resolveRedisTools(ctx context.Context, item aiengine.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (aiengine.ToolResult, error))) error {
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "agent") {
		return fmt.Errorf("Redis agent resources must use the MCP provider")
	}
	connection, err := s.redisConnection(ctx, item)
	if err != nil {
		return err
	}
	register := func(name, desc string, fn func(context.Context, rt.ConnectionInput) (any, error)) {
		add(name, desc, redisDirectSchema(), func(runCtx context.Context, _ map[string]any) (aiengine.ToolResult, error) {
			out, e := fn(runCtx, connection)
			if e != nil {
				return aiengine.ToolResult{}, redisToolError(name, e)
			}
			return aiengine.ToolResult{Output: out, Untrusted: true}, nil
		})
	}
	for _, tool := range rt.ListTools() {
		switch tool.Name {
		case "redis_health":
			register(tool.Name, tool.Description, func(c context.Context, i rt.ConnectionInput) (any, error) { return rt.Health(c, i) })
		case "redis_memory":
			register(tool.Name, tool.Description, func(c context.Context, i rt.ConnectionInput) (any, error) { return rt.Memory(c, i) })
		case "redis_clients":
			register(tool.Name, tool.Description, func(c context.Context, i rt.ConnectionInput) (any, error) { return rt.Clients(c, i) })
		case "redis_replication":
			register(tool.Name, tool.Description, func(c context.Context, i rt.ConnectionInput) (any, error) { return rt.Replication(c, i) })
		case "redis_slowlog":
			register(tool.Name, tool.Description, func(c context.Context, i rt.ConnectionInput) (any, error) { return rt.Slowlog(c, i) })
		case "redis_database_info":
			register(tool.Name, tool.Description, func(c context.Context, i rt.ConnectionInput) (any, error) { return rt.DatabaseInfo(c, i) })
		}
	}
	return nil
}
func redisDirectSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":false}`)
}
func redisToolError(name string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", name, err)
}
func (s *Service) redisConnection(ctx context.Context, item aiengine.ContextResource) (rt.ConnectionInput, error) {
	in := rt.ConnectionInput{Host: stringValue(item.Config, "host"), Port: intValue(item.Config, "port"), Database: intValue(item.Config, "database"), TimeoutSeconds: intValue(item.Config, "timeout_seconds")}
	if item.CredentialID == nil || strings.TrimSpace(*item.CredentialID) == "" {
		return in, nil
	}
	if s.credentials == nil {
		return in, fmt.Errorf("credential service is unavailable")
	}
	secret, e := s.credentials.RevealLinked(ctx, *item.CredentialID)
	if e != nil {
		return in, e
	}
	var values map[string]any
	if e := json.Unmarshal(secret, &values); e != nil {
		return in, fmt.Errorf("decode Redis credential: %w", e)
	}
	in.Username = stringValue(values, "username")
	in.Password = stringValue(values, "password")
	if in.Host == "" {
		in.Host = stringValue(values, "host")
	}
	if in.Port == 0 {
		in.Port = intValue(values, "port")
	}
	if in.Database == 0 {
		in.Database = intValue(values, "database")
	}
	if in.TimeoutSeconds == 0 {
		in.TimeoutSeconds = intValue(values, "timeout_seconds")
	}
	return in, nil
}
