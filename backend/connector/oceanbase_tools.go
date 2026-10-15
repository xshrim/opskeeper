package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opskeeper/backend/engine"
	ot "opskeeper/backend/tool/oceanbase"
)

func (s *Service) resolveOceanBaseTools(ctx context.Context, item engine.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (engine.ToolResult, error))) error {
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "agent") {
		return fmt.Errorf("OceanBase agent resources must use the MCP provider")
	}
	connection, err := s.oceanBaseConnection(ctx, item)
	if err != nil {
		return err
	}
	register := func(name, desc string, fn func(context.Context, ot.ConnectionInput) (any, error)) {
		add(name, desc, oceanBaseDirectSchema(), func(c context.Context, _ map[string]any) (engine.ToolResult, error) {
			out, e := fn(c, connection)
			if e != nil {
				return engine.ToolResult{}, oceanBaseError(name, e)
			}
			return engine.ToolResult{Output: out, Untrusted: true}, nil
		})
	}
	for _, tool := range ot.ListTools() {
		switch tool.Name {
		case "oceanbase_health":
			register(tool.Name, tool.Description, func(c context.Context, in ot.ConnectionInput) (any, error) { return ot.Health(c, in) })
		case "oceanbase_status":
			register(tool.Name, tool.Description, func(c context.Context, in ot.ConnectionInput) (any, error) { return ot.Status(c, in) })
		case "oceanbase_performance":
			register(tool.Name, tool.Description, func(c context.Context, in ot.ConnectionInput) (any, error) { return ot.Performance(c, in) })
		case "oceanbase_database_info":
			register(tool.Name, tool.Description, func(c context.Context, in ot.ConnectionInput) (any, error) { return ot.DatabaseInfo(c, in) })
		case "oceanbase_tables":
			register(tool.Name, tool.Description, func(c context.Context, in ot.ConnectionInput) (any, error) { return ot.Tables(c, in) })
		case "oceanbase_table_columns":
			add(tool.Name, tool.Description, oceanBaseColumnsSchema(), func(c context.Context, args map[string]any) (engine.ToolResult, error) {
				var input ot.TableColumnsInput
				raw, _ := json.Marshal(args)
				if e := json.Unmarshal(raw, &input); e != nil {
					return engine.ToolResult{}, e
				}
				out, e := ot.TableColumns(c, connection, input)
				if e != nil {
					return engine.ToolResult{}, oceanBaseError(tool.Name, e)
				}
				return engine.ToolResult{Output: out, Untrusted: true}, nil
			})
		case "oceanbase_table_structure":
			add(tool.Name, tool.Description, oceanBaseColumnsSchema(), func(c context.Context, args map[string]any) (engine.ToolResult, error) {
				var input ot.TableColumnsInput
				raw, _ := json.Marshal(args)
				if e := json.Unmarshal(raw, &input); e != nil {
					return engine.ToolResult{}, e
				}
				out, e := ot.TableStructure(c, connection, input)
				if e != nil {
					return engine.ToolResult{}, oceanBaseError(tool.Name, e)
				}
				return engine.ToolResult{Output: out, Untrusted: true}, nil
			})
		}
	}
	return nil
}
func oceanBaseDirectSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":false}`)
}
func oceanBaseColumnsSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","required":["schema","table"],"properties":{"schema":{"type":"string","minLength":1,"maxLength":128},"table":{"type":"string","minLength":1,"maxLength":128}},"additionalProperties":false}`)
}
func (s *Service) oceanBaseConnection(ctx context.Context, item engine.ContextResource) (ot.ConnectionInput, error) {
	in := ot.ConnectionInput{Host: stringValue(item.Config, "host"), Port: intValue(item.Config, "port"), Mode: stringValue(item.Config, "mode"), Database: stringValue(item.Config, "database"), ServiceName: stringValue(item.Config, "service_name"), TimeoutSeconds: intValue(item.Config, "timeout_seconds")}
	secret, configured, err := s.resourceSecret(ctx, item.ID)
	if err != nil {
		return in, err
	}
	if !configured {
		return in, fmt.Errorf("OceanBase credential is required")
	}
	var values map[string]any
	if err = json.Unmarshal(secret, &values); err != nil {
		return in, fmt.Errorf("decode OceanBase credential: %w", err)
	}
	in.Username, in.Password = stringValue(values, "username"), stringValue(values, "password")
	if in.Host == "" {
		in.Host = stringValue(values, "host")
	}
	if in.Port == 0 {
		in.Port = intValue(values, "port")
	}
	if in.Database == "" {
		in.Database = stringValue(values, "database")
	}
	if in.ServiceName == "" {
		in.ServiceName = stringValue(values, "service_name")
	}
	return in, nil
}
