package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"opskeeper/backend/aiengine"
	mt "opskeeper/backend/tool/mysql"
	"strings"
)

func (s *Service) resolveMySQLTools(ctx context.Context, item aiengine.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (aiengine.ToolResult, error))) error {
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "agent") {
		return fmt.Errorf("MySQL agent resources must use the MCP provider")
	}
	connection, err := s.mySQLConnection(ctx, item)
	if err != nil {
		return err
	}
	register := func(name, desc string, fn func(context.Context, mt.ConnectionInput) (any, error)) {
		add(name, desc, mysqlDirectSchema(), func(c context.Context, _ map[string]any) (aiengine.ToolResult, error) {
			out, e := fn(c, connection)
			if e != nil {
				return aiengine.ToolResult{}, mysqlError(name, e)
			}
			return aiengine.ToolResult{Output: out, Untrusted: true}, nil
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
			add(tool.Name, tool.Description, mysqlColumnsSchema(), func(c context.Context, args map[string]any) (aiengine.ToolResult, error) {
				var in mt.TableColumnsInput
				raw, _ := json.Marshal(args)
				if e := json.Unmarshal(raw, &in); e != nil {
					return aiengine.ToolResult{}, e
				}
				out, e := mt.TableColumns(c, connection, in)
				if e != nil {
					return aiengine.ToolResult{}, mysqlError(tool.Name, e)
				}
				return aiengine.ToolResult{Output: out, Untrusted: true}, nil
			})
		case "mysql_table_structure":
			add(tool.Name, tool.Description, mysqlColumnsSchema(), func(c context.Context, args map[string]any) (aiengine.ToolResult, error) {
				var in mt.TableColumnsInput
				raw, _ := json.Marshal(args)
				if e := json.Unmarshal(raw, &in); e != nil {
					return aiengine.ToolResult{}, e
				}
				out, e := mt.TableStructure(c, connection, in)
				if e != nil {
					return aiengine.ToolResult{}, mysqlError(tool.Name, e)
				}
				return aiengine.ToolResult{Output: out, Untrusted: true}, nil
			})
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
func (s *Service) mySQLConnection(ctx context.Context, item aiengine.ContextResource) (mt.ConnectionInput, error) {
	in := mt.ConnectionInput{Host: stringValue(item.Config, "host"), Database: stringValue(item.Config, "database"), Port: intValue(item.Config, "port"), TimeoutSeconds: intValue(item.Config, "timeout_seconds")}
	if item.CredentialID == nil || strings.TrimSpace(*item.CredentialID) == "" {
		return in, fmt.Errorf("MySQL credential is required")
	}
	if s.credentials == nil {
		return in, fmt.Errorf("credential service is unavailable")
	}
	secret, err := s.credentials.RevealLinked(ctx, *item.CredentialID)
	if err != nil {
		return in, err
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
