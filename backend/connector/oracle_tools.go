package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opskeeper/backend/aiengine"
	ot "opskeeper/backend/tool/oracle"
)

func (s *Service) resolveOracleTools(ctx context.Context, item aiengine.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (aiengine.ToolResult, error))) error {
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "agent") {
		return fmt.Errorf("Oracle agent resources must use the MCP provider")
	}
	connection, err := s.oracleConnection(ctx, item)
	if err != nil {
		return err
	}
	register := func(name, desc string, fn func(context.Context, ot.ConnectionInput) (any, error)) {
		add(name, desc, oracleDirectSchema(), func(c context.Context, _ map[string]any) (aiengine.ToolResult, error) {
			out, e := fn(c, connection)
			if e != nil {
				return aiengine.ToolResult{}, oracleError(name, e)
			}
			return aiengine.ToolResult{Output: out, Untrusted: true}, nil
		})
	}
	for _, tool := range ot.ListTools() {
		switch tool.Name {
		case "oracle_health":
			register(tool.Name, tool.Description, func(c context.Context, i ot.ConnectionInput) (any, error) { return ot.Health(c, i) })
		case "oracle_status":
			register(tool.Name, tool.Description, func(c context.Context, i ot.ConnectionInput) (any, error) { return ot.Status(c, i) })
		case "oracle_performance":
			register(tool.Name, tool.Description, func(c context.Context, i ot.ConnectionInput) (any, error) { return ot.Performance(c, i) })
		case "oracle_database_info":
			register(tool.Name, tool.Description, func(c context.Context, i ot.ConnectionInput) (any, error) { return ot.DatabaseInfo(c, i) })
		case "oracle_tables":
			register(tool.Name, tool.Description, func(c context.Context, i ot.ConnectionInput) (any, error) { return ot.Tables(c, i) })
		case "oracle_table_columns":
			add(tool.Name, tool.Description, oracleColumnsSchema(), func(c context.Context, args map[string]any) (aiengine.ToolResult, error) {
				var in ot.TableColumnsInput
				raw, _ := json.Marshal(args)
				if e := json.Unmarshal(raw, &in); e != nil {
					return aiengine.ToolResult{}, e
				}
				out, e := ot.TableColumns(c, connection, in)
				if e != nil {
					return aiengine.ToolResult{}, oracleError(tool.Name, e)
				}
				return aiengine.ToolResult{Output: out, Untrusted: true}, nil
			})
		case "oracle_table_structure":
			add(tool.Name, tool.Description, oracleColumnsSchema(), func(c context.Context, args map[string]any) (aiengine.ToolResult, error) {
				var in ot.TableColumnsInput
				raw, _ := json.Marshal(args)
				if e := json.Unmarshal(raw, &in); e != nil {
					return aiengine.ToolResult{}, e
				}
				out, e := ot.TableStructure(c, connection, in)
				if e != nil {
					return aiengine.ToolResult{}, oracleError(tool.Name, e)
				}
				return aiengine.ToolResult{Output: out, Untrusted: true}, nil
			})
		}
	}
	return nil
}

func oracleDirectSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":false}`)
}
func oracleColumnsSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","required":["schema","table"],"properties":{"schema":{"type":"string","minLength":1,"maxLength":128},"table":{"type":"string","minLength":1,"maxLength":128}},"additionalProperties":false}`)
}

func (s *Service) oracleConnection(ctx context.Context, item aiengine.ContextResource) (ot.ConnectionInput, error) {
	in := ot.ConnectionInput{Host: stringValue(item.Config, "host"), Port: intValue(item.Config, "port"), ServiceName: stringValue(item.Config, "service_name"), SID: stringValue(item.Config, "sid"), TimeoutSeconds: intValue(item.Config, "timeout_seconds"), TLS: configBool(item.Config, "tls")}
	if item.CredentialID == nil || strings.TrimSpace(*item.CredentialID) == "" {
		return in, fmt.Errorf("Oracle credential is required")
	}
	if s.credentials == nil {
		return in, fmt.Errorf("credential service is unavailable")
	}
	secret, e := s.credentials.RevealLinked(ctx, *item.CredentialID)
	if e != nil {
		return in, e
	}
	var values map[string]any
	if e = json.Unmarshal(secret, &values); e != nil {
		return in, fmt.Errorf("decode Oracle credential: %w", e)
	}
	in.Username, in.Password = stringValue(values, "username"), stringValue(values, "password")
	if in.Host == "" {
		in.Host = stringValue(values, "host")
	}
	if in.Port == 0 {
		in.Port = intValue(values, "port")
	}
	if in.ServiceName == "" {
		in.ServiceName = stringValue(values, "service_name")
	}
	if in.SID == "" {
		in.SID = stringValue(values, "sid")
	}
	return in, nil
}
