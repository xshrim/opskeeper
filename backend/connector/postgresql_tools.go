package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opskeeper/backend/aiengine"
	pt "opskeeper/backend/tool/postgresql"
)

func (s *Service) resolvePostgreSQLTools(ctx context.Context, item aiengine.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (aiengine.ToolResult, error))) error {
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "agent") {
		return fmt.Errorf("PostgreSQL agent resources must use the MCP provider")
	}
	connection, err := s.postgreSQLConnection(ctx, item)
	if err != nil {
		return err
	}
	register := func(name, description string, fn func(context.Context, pt.ConnectionInput) (any, error)) {
		add(name, description, postgreSQLDirectSchema(), func(runCtx context.Context, _ map[string]any) (aiengine.ToolResult, error) {
			out, callErr := fn(runCtx, connection)
			if callErr != nil {
				return aiengine.ToolResult{}, postgreSQLError(name, callErr)
			}
			return aiengine.ToolResult{Output: out, Untrusted: true}, nil
		})
	}
	for _, tool := range pt.ListTools() {
		switch tool.Name {
		case "postgresql_health":
			register(tool.Name, tool.Description, func(c context.Context, i pt.ConnectionInput) (any, error) { return pt.Health(c, i) })
		case "postgresql_sessions":
			register(tool.Name, tool.Description, func(c context.Context, i pt.ConnectionInput) (any, error) { return pt.Sessions(c, i) })
		case "postgresql_long_running_queries":
			register(tool.Name, tool.Description, func(c context.Context, i pt.ConnectionInput) (any, error) { return pt.LongRunningQueries(c, i) })
		case "postgresql_locks":
			register(tool.Name, tool.Description, func(c context.Context, i pt.ConnectionInput) (any, error) { return pt.Locks(c, i) })
		case "postgresql_replication":
			register(tool.Name, tool.Description, func(c context.Context, i pt.ConnectionInput) (any, error) { return pt.Replication(c, i) })
		case "postgresql_capacity":
			register(tool.Name, tool.Description, func(c context.Context, i pt.ConnectionInput) (any, error) { return pt.Capacity(c, i) })
		case "postgresql_tables":
			register(tool.Name, tool.Description, func(c context.Context, i pt.ConnectionInput) (any, error) { return pt.Tables(c, i) })
		case "postgresql_performance":
			register(tool.Name, tool.Description, func(c context.Context, i pt.ConnectionInput) (any, error) { return pt.Performance(c, i) })
		case "postgresql_vacuum":
			register(tool.Name, tool.Description, func(c context.Context, i pt.ConnectionInput) (any, error) { return pt.Vacuum(c, i) })
		case "postgresql_extensions":
			register(tool.Name, tool.Description, func(c context.Context, i pt.ConnectionInput) (any, error) { return pt.Extensions(c, i) })
		case "postgresql_database_info":
			register(tool.Name, tool.Description, func(c context.Context, i pt.ConnectionInput) (any, error) { return pt.DatabaseInfo(c, i) })
		case "postgresql_table_columns":
			add(tool.Name, tool.Description, postgreSQLColumnsSchema(), func(runCtx context.Context, args map[string]any) (aiengine.ToolResult, error) {
				var input pt.TableColumnsInput
				raw, _ := json.Marshal(args)
				if err := json.Unmarshal(raw, &input); err != nil {
					return aiengine.ToolResult{}, err
				}
				out, err := pt.TableColumns(runCtx, connection, input)
				if err != nil {
					return aiengine.ToolResult{}, postgreSQLError(tool.Name, err)
				}
				return aiengine.ToolResult{Output: out, Untrusted: true}, nil
			})
		}
	}
	return nil
}

func postgreSQLColumnsSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","required":["schema","table"],"properties":{"schema":{"type":"string"},"table":{"type":"string"}},"additionalProperties":false}`)
}

func postgreSQLDirectSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":false}`)
}

func (s *Service) postgreSQLConnection(ctx context.Context, item aiengine.ContextResource) (pt.ConnectionInput, error) {
	input := pt.ConnectionInput{Host: stringValue(item.Config, "host"), Database: stringValue(item.Config, "database"), Port: intValue(item.Config, "port"), TimeoutSeconds: intValue(item.Config, "timeout_seconds")}
	if item.CredentialID == nil || strings.TrimSpace(*item.CredentialID) == "" {
		return input, fmt.Errorf("PostgreSQL credential is required")
	}
	if s.credentials == nil {
		return input, fmt.Errorf("credential service is unavailable")
	}
	secret, err := s.credentials.RevealLinked(ctx, *item.CredentialID)
	if err != nil {
		return input, err
	}
	var values map[string]any
	if err := json.Unmarshal(secret, &values); err != nil {
		return input, fmt.Errorf("decode PostgreSQL credential: %w", err)
	}
	input.Username = stringValue(values, "username")
	input.Password = stringValue(values, "password")
	if input.Host == "" {
		input.Host = stringValue(values, "host")
	}
	if input.Database == "" {
		input.Database = stringValue(values, "database")
	}
	if input.Port == 0 {
		input.Port = intValue(values, "port")
	}
	if input.TimeoutSeconds == 0 {
		input.TimeoutSeconds = intValue(values, "timeout_seconds")
	}
	return input, nil
}
