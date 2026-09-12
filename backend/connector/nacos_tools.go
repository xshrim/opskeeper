package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opskeeper/backend/aiengine"
	nt "opskeeper/backend/tool/nacos"
)

func (s *Service) resolveNacosTools(ctx context.Context, item aiengine.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (aiengine.ToolResult, error))) error {
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "agent") {
		return fmt.Errorf("Nacos agent resources must use the MCP provider")
	}
	connection, err := s.nacosConnection(ctx, item)
	if err != nil {
		return err
	}
	register := func(name, description string, schema json.RawMessage, fn func(context.Context, map[string]any) (any, error)) {
		add(name, description, schema, func(runCtx context.Context, args map[string]any) (aiengine.ToolResult, error) {
			out, e := fn(runCtx, args)
			if e != nil {
				return aiengine.ToolResult{}, fmt.Errorf("%s: %w", name, e)
			}
			return aiengine.ToolResult{Output: out, Untrusted: true}, nil
		})
	}
	for _, tool := range nt.ListTools() {
		switch tool.Name {
		case "nacos_server_state":
			register(tool.Name, tool.Description, nacosSchema(nil), func(c context.Context, _ map[string]any) (any, error) { return nt.Health(c, connection) })
		case "nacos_namespaces":
			register(tool.Name, tool.Description, nacosSchema(nil), func(c context.Context, _ map[string]any) (any, error) { return nt.Namespaces(c, connection) })
		case "nacos_services":
			register(tool.Name, tool.Description, nacosSchema(map[string]any{"namespace_id": map[string]any{"type": "string"}, "group_name": map[string]any{"type": "string"}, "page": map[string]any{"type": "integer"}, "page_size": map[string]any{"type": "integer"}}), func(c context.Context, a map[string]any) (any, error) {
				var i nt.ServicesInput
				return decodeNacos(c, a, &i, func() (nt.JSONOutput, error) { return nt.Services(c, connection, i) })
			})
		case "nacos_service_instances":
			register(tool.Name, tool.Description, nacosSchema(map[string]any{"service_name": map[string]any{"type": "string"}, "group_name": map[string]any{"type": "string"}, "namespace_id": map[string]any{"type": "string"}, "healthy_only": map[string]any{"type": "boolean"}, "__required": []string{"service_name"}}), func(c context.Context, a map[string]any) (any, error) {
				var i nt.ServiceInstancesInput
				return decodeNacos(c, a, &i, func() (nt.JSONOutput, error) { return nt.ServiceInstances(c, connection, i) })
			})
		case "nacos_configs":
			register(tool.Name, tool.Description, nacosSchema(map[string]any{"namespace_id": map[string]any{"type": "string"}, "group_name": map[string]any{"type": "string"}, "search": map[string]any{"type": "string"}, "page": map[string]any{"type": "integer"}, "page_size": map[string]any{"type": "integer"}}), func(c context.Context, a map[string]any) (any, error) {
				var i nt.ConfigsInput
				return decodeNacos(c, a, &i, func() (nt.JSONOutput, error) { return nt.Configs(c, connection, i) })
			})
		case "nacos_config_detail":
			register(tool.Name, tool.Description, nacosSchema(map[string]any{"data_id": map[string]any{"type": "string"}, "group_name": map[string]any{"type": "string"}, "namespace_id": map[string]any{"type": "string"}, "__required": []string{"data_id"}}), func(c context.Context, a map[string]any) (any, error) {
				var i nt.ConfigDetailInput
				return decodeNacos(c, a, &i, func() (nt.JSONOutput, error) { return nt.ConfigDetail(c, connection, i) })
			})
		}
	}
	return nil
}
func nacosSchema(extra map[string]any) json.RawMessage {
	schema := nt.InputSchema(extra)
	if p, ok := schema["properties"].(map[string]any); ok {
		for _, k := range []string{"host", "port", "scheme", "context_path", "username", "password", "access_token", "timeout_seconds", "__required"} {
			delete(p, k)
		}
	}
	if req, ok := extra["__required"]; ok {
		schema["required"] = req
	}
	b, _ := json.Marshal(schema)
	return b
}
func decodeNacos[T any](ctx context.Context, a map[string]any, out *T, fn func() (nt.JSONOutput, error)) (any, error) {
	b, e := json.Marshal(a)
	if e != nil {
		return nil, e
	}
	if e = json.Unmarshal(b, out); e != nil {
		return nil, e
	}
	return fn()
}
func (s *Service) nacosConnection(ctx context.Context, item aiengine.ContextResource) (nt.ConnectionInput, error) {
	in := nt.ConnectionInput{Host: stringValue(item.Config, "host"), Port: intValue(item.Config, "port"), Scheme: stringValue(item.Config, "scheme"), ContextPath: stringValue(item.Config, "context_path"), TimeoutSeconds: intValue(item.Config, "timeout_seconds")}
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
	if e = json.Unmarshal(secret, &values); e != nil {
		return in, fmt.Errorf("decode Nacos credential: %w", e)
	}
	for _, k := range []string{"username", "password", "access_token"} {
		if v := stringValue(values, k); v != "" {
			switch k {
			case "username":
				in.Username = v
			case "password":
				in.Password = v
			case "access_token":
				in.AccessToken = v
			}
		}
	}
	if in.Host == "" {
		in.Host = stringValue(values, "host")
	}
	if in.Port == 0 {
		in.Port = intValue(values, "port")
	}
	return in, nil
}
