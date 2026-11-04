package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opskeeper/backend/engine"
	pt "opskeeper/backend/tool/postgresql"
)

func (s *Service) resolvePostgreSQLTools(ctx context.Context, item engine.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (engine.ToolResult, error))) error {
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "agent") {
		return fmt.Errorf("PostgreSQL agent resources must use the MCP provider")
	}
	connection, err := s.postgreSQLConnection(ctx, item)
	if err != nil {
		return err
	}
	register := func(name, description string, fn func(context.Context, pt.ConnectionInput) (any, error)) {
		add(name, description, postgreSQLDirectSchema(), func(runCtx context.Context, _ map[string]any) (engine.ToolResult, error) {
			out, callErr := fn(runCtx, connection)
			if callErr != nil {
				return engine.ToolResult{}, postgreSQLError(name, callErr)
			}
			return engine.ToolResult{Output: out, Untrusted: true}, nil
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
			addPostgreSQLTableTool(add, tool.Name, tool.Description, postgreSQLColumnsSchema(), connection, false)
		case "postgresql_table_structure":
			addPostgreSQLTableTool(add, tool.Name, tool.Description, postgreSQLColumnsSchema(), connection, true)
		case "postgresql_table_schema_lookup":
			add(tool.Name, tool.Description, postgreSQLLookupSchema(), func(runCtx context.Context, args map[string]any) (engine.ToolResult, error) {
				var input pt.SchemaLookupInput
				if err := decodePostgreSQLArgs(args, &input); err != nil {
					return engine.ToolResult{}, err
				}
				out, err := pt.TableSchemaLookup(runCtx, connection, input)
				if err != nil {
					return engine.ToolResult{}, postgreSQLError(tool.Name, err)
				}
				return engine.ToolResult{Output: out, Untrusted: true}, nil
			})
		case "postgresql_slow_queries":
			add(tool.Name, tool.Description, postgreSQLSlowQueriesSchema(), func(runCtx context.Context, args map[string]any) (engine.ToolResult, error) {
				var input pt.SlowQueriesInput
				if err := decodePostgreSQLArgs(args, &input); err != nil {
					return engine.ToolResult{}, err
				}
				out, err := pt.SlowQueries(runCtx, connection, input)
				if err != nil {
					return engine.ToolResult{}, postgreSQLError(tool.Name, err)
				}
				return engine.ToolResult{Output: out, Untrusted: true}, nil
			})
		case "postgresql_explain":
			add(tool.Name, tool.Description, postgreSQLExplainSchema(), func(runCtx context.Context, args map[string]any) (engine.ToolResult, error) {
				var input pt.ExplainInput
				if err := decodePostgreSQLArgs(args, &input); err != nil {
					return engine.ToolResult{}, err
				}
				out, err := pt.Explain(runCtx, connection, input)
				if err != nil {
					return engine.ToolResult{}, postgreSQLError(tool.Name, err)
				}
				return engine.ToolResult{Output: out, Untrusted: true}, nil
			})
		case "postgresql_active_queries":
			register(tool.Name, tool.Description, func(c context.Context, i pt.ConnectionInput) (any, error) { return pt.ActiveQueries(c, i) })
		case "postgresql_lock_waits":
			register(tool.Name, tool.Description, func(c context.Context, i pt.ConnectionInput) (any, error) { return pt.LockWaits(c, i) })
		case "postgresql_vacuum_table_stats":
			register(tool.Name, tool.Description, func(c context.Context, i pt.ConnectionInput) (any, error) { return pt.VacuumTableStats(c, i) })
		case "postgresql_connection_stats":
			register(tool.Name, tool.Description, func(c context.Context, i pt.ConnectionInput) (any, error) { return pt.ConnectionStats(c, i) })
		case "postgresql_connection_groups":
			register(tool.Name, tool.Description, func(c context.Context, i pt.ConnectionInput) (any, error) { return pt.ConnectionGroups(c, i) })
		case "postgresql_idle_sessions":
			register(tool.Name, tool.Description, func(c context.Context, i pt.ConnectionInput) (any, error) { return pt.IdleSessions(c, i) })
		}
	}
	return nil
}

func decodePostgreSQLArgs(args map[string]any, out any) error {
	raw, err := json.Marshal(args)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func addPostgreSQLTableTool(add func(string, string, json.RawMessage, func(context.Context, map[string]any) (engine.ToolResult, error)), name, description string, schema json.RawMessage, connection pt.ConnectionInput, structure bool) {
	add(name, description, schema, func(runCtx context.Context, args map[string]any) (engine.ToolResult, error) {
		var input pt.TableColumnsInput
		if err := decodePostgreSQLArgs(args, &input); err != nil {
			return engine.ToolResult{}, err
		}
		var out any
		var err error
		if structure {
			out, err = pt.TableStructure(runCtx, connection, input)
		} else {
			out, err = pt.TableColumns(runCtx, connection, input)
		}
		if err != nil {
			return engine.ToolResult{}, postgreSQLError(name, err)
		}
		return engine.ToolResult{Output: out, Untrusted: true}, nil
	})
}

func postgreSQLColumnsSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","required":["schema","table"],"properties":{"schema":{"type":"string"},"table":{"type":"string"}},"additionalProperties":false}`)
}

func postgreSQLLookupSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","required":["table"],"properties":{"table":{"type":"string"}},"additionalProperties":false}`)
}
func postgreSQLSlowQueriesSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer","minimum":1,"maximum":100},"min_total_ms":{"type":"number","minimum":0}},"additionalProperties":false}`)
}
func postgreSQLExplainSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","required":["query"],"properties":{"schema":{"type":"string"},"query":{"type":"string","minLength":1,"maxLength":10000}},"additionalProperties":false}`)
}

func postgreSQLDirectSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":false}`)
}

func (s *Service) postgreSQLConnection(ctx context.Context, item engine.ContextResource) (pt.ConnectionInput, error) {
	input := pt.ConnectionInput{Host: stringValue(item.Config, "host"), Database: stringValue(item.Config, "database"), Port: intValue(item.Config, "port"), TimeoutSeconds: intValue(item.Config, "timeout_seconds")}
	secret, configured, err := s.resourceSecret(ctx, item.ID)
	if err != nil {
		return input, err
	}
	if !configured {
		return input, fmt.Errorf("PostgreSQL credential is required")
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
