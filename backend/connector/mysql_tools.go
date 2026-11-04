package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"opskeeper/backend/engine"
	mt "opskeeper/backend/tool/mysql"
	"strings"
)

func (s *Service) resolveMySQLTools(ctx context.Context, item engine.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (engine.ToolResult, error))) error {
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "agent") {
		return fmt.Errorf("MySQL agent resources must use the MCP provider")
	}
	connection, err := s.mySQLConnection(ctx, item)
	if err != nil {
		return err
	}
	register := func(name, desc string, fn func(context.Context, mt.ConnectionInput) (any, error)) {
		add(name, desc, mysqlDirectSchema(), func(c context.Context, _ map[string]any) (engine.ToolResult, error) {
			out, e := fn(c, connection)
			if e != nil {
				return engine.ToolResult{}, mysqlError(name, e)
			}
			return engine.ToolResult{Output: out, Untrusted: true}, nil
		})
	}
	for _, tool := range mt.ListTools() {
		switch tool.Name {
		case "mysql_health":
			register(tool.Name, tool.Description, func(c context.Context, i mt.ConnectionInput) (any, error) { return mt.Health(c, i) })
		case "mysql_status":
			register(tool.Name, tool.Description, func(c context.Context, i mt.ConnectionInput) (any, error) { return mt.Status(c, i) })
		case "mysql_performance":
			register(tool.Name, tool.Description, func(c context.Context, i mt.ConnectionInput) (any, error) { return mt.Performance(c, i) })
		case "mysql_tables":
			register(tool.Name, tool.Description, func(c context.Context, i mt.ConnectionInput) (any, error) { return mt.Tables(c, i) })
		case "mysql_database_info":
			register(tool.Name, tool.Description, func(c context.Context, i mt.ConnectionInput) (any, error) { return mt.DatabaseInfo(c, i) })
		case "mysql_table_columns":
			add(tool.Name, tool.Description, mysqlColumnsSchema(), func(c context.Context, args map[string]any) (engine.ToolResult, error) {
				var in mt.TableColumnsInput
				raw, _ := json.Marshal(args)
				if e := json.Unmarshal(raw, &in); e != nil {
					return engine.ToolResult{}, e
				}
				out, e := mt.TableColumns(c, connection, in)
				if e != nil {
					return engine.ToolResult{}, mysqlError(tool.Name, e)
				}
				return engine.ToolResult{Output: out, Untrusted: true}, nil
			})
		case "mysql_table_structure":
			add(tool.Name, tool.Description, mysqlColumnsSchema(), func(c context.Context, args map[string]any) (engine.ToolResult, error) {
				var in mt.TableColumnsInput
				raw, _ := json.Marshal(args)
				if e := json.Unmarshal(raw, &in); e != nil {
					return engine.ToolResult{}, e
				}
				out, e := mt.TableStructure(c, connection, in)
				if e != nil {
					return engine.ToolResult{}, mysqlError(tool.Name, e)
				}
				return engine.ToolResult{Output: out, Untrusted: true}, nil
			})
		case "mysql_slow_queries":
			add(tool.Name, tool.Description, mysqlSlowQueriesSchema(), func(c context.Context, args map[string]any) (engine.ToolResult, error) {
				var input mt.SlowQueriesInput
				raw, _ := json.Marshal(args)
				if e := json.Unmarshal(raw, &input); e != nil {
					return engine.ToolResult{}, e
				}
				out, e := mt.SlowQueries(c, connection, input)
				if e != nil {
					return engine.ToolResult{}, mysqlError(tool.Name, e)
				}
				return engine.ToolResult{Output: out, Untrusted: true}, nil
			})
		case "mysql_active_queries":
			register(tool.Name, tool.Description, func(c context.Context, i mt.ConnectionInput) (any, error) { return mt.ActiveQueries(c, i) })
		case "mysql_lock_waits":
			register(tool.Name, tool.Description, func(c context.Context, i mt.ConnectionInput) (any, error) { return mt.LockWaits(c, i) })
		case "mysql_connection_stats":
			register(tool.Name, tool.Description, func(c context.Context, i mt.ConnectionInput) (any, error) { return mt.ConnectionStats(c, i) })
		}
	}
	return nil
}
func mysqlDirectSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":false}`)
}
func mysqlColumnsSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","required":["schema","table"],"properties":{"schema":{"type":"string"},"table":{"type":"string"}},"additionalProperties":false}`)
}
func mysqlSlowQueriesSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer","minimum":1,"maximum":100},"min_total_ms":{"type":"number","minimum":0}},"additionalProperties":false}`)
}
func (s *Service) mySQLConnection(ctx context.Context, item engine.ContextResource) (mt.ConnectionInput, error) {
	in := mt.ConnectionInput{Host: stringValue(item.Config, "host"), Database: stringValue(item.Config, "database"), Port: intValue(item.Config, "port"), TimeoutSeconds: intValue(item.Config, "timeout_seconds")}
	secret, configured, err := s.resourceSecret(ctx, item.ID)
	if err != nil {
		return in, err
	}
	if !configured {
		return in, fmt.Errorf("MySQL credential is required")
	}
	var values map[string]any
	if err = json.Unmarshal(secret, &values); err != nil {
		return in, fmt.Errorf("decode MySQL credential: %w", err)
	}
	in.Username, in.Password = stringValue(values, "username"), stringValue(values, "password")
	if in.Host == "" {
		in.Host = stringValue(values, "host")
	}
	if in.Database == "" {
		in.Database = stringValue(values, "database")
	}
	if in.Port == 0 {
		in.Port = intValue(values, "port")
	}
	if in.TimeoutSeconds == 0 {
		in.TimeoutSeconds = intValue(values, "timeout_seconds")
	}
	return in, nil
}
