package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"opskeeper/backend/aiengine"
	es "opskeeper/backend/tool/elasticsearch"
	"strings"
)

func (s *Service) resolveElasticsearchTools(ctx context.Context, item aiengine.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (aiengine.ToolResult, error))) error {
	if strings.EqualFold(item.Subtype, "agent") {
		return fmt.Errorf("Elasticsearch agent resources must use the MCP provider")
	}
	in := es.ConnectionInput{URL: stringValue(item.Config, "url"), TLSInsecure: configBool(item.Config, "tls_insecure"), TimeoutSeconds: intValue(item.Config, "timeout_seconds")}
	if item.CredentialID != nil && strings.TrimSpace(*item.CredentialID) != "" {
		if s.credentials == nil {
			return fmt.Errorf("credential service is unavailable")
		}
		raw, e := s.credentials.RevealLinked(ctx, *item.CredentialID)
		if e != nil {
			return e
		}
		var v map[string]any
		_ = json.Unmarshal(raw, &v)
		in.Username = stringValue(v, "username")
		in.Password = stringValue(v, "password")
	}
	c, e := es.New(in)
	if e != nil {
		return e
	}
	for _, t := range es.ListTools() {
		name := t.Name
		add(name, t.Description, esSchema(name), func(rc context.Context, a map[string]any) (aiengine.ToolResult, error) {
			var out any
			var er error
			switch name {
			case "elasticsearch_health":
				out, er = c.Health(rc)
			case "elasticsearch_cluster_info":
				out, er = c.ClusterInfo(rc)
			case "elasticsearch_nodes":
				out, er = c.Nodes(rc)
			case "elasticsearch_indices":
				out, er = c.Indices(rc)
			case "elasticsearch_shards":
				out, er = c.Shards(rc)
			case "elasticsearch_settings":
				out, er = c.Settings(rc)
			case "elasticsearch_index_mapping":
				out, er = c.IndexMapping(rc, stringArg(a, "index"))
			}
			if er != nil {
				return aiengine.ToolResult{}, fmt.Errorf("%s: %w", name, er)
			}
			return aiengine.ToolResult{Output: out, Untrusted: true}, nil
		})
	}
	return nil
}
func esSchema(name string) json.RawMessage {
	if name == "elasticsearch_index_mapping" {
		return json.RawMessage(`{"type":"object","required":["index"],"properties":{"index":{"type":"string","minLength":1}},"additionalProperties":false}`)
	}
	return json.RawMessage(`{"type":"object","additionalProperties":false}`)
}
