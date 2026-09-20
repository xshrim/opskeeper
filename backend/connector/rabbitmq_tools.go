package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"opskeeper/backend/engine"
	rmq "opskeeper/backend/tool/rabbitmq"
	"strings"
)

func (s *Service) resolveRabbitMQTools(ctx context.Context, item engine.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (engine.ToolResult, error))) error {
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "agent") {
		return fmt.Errorf("RabbitMQ agent resources must use the MCP provider")
	}
	in := rmq.ConnectionInput{URL: stringValue(item.Config, "url"), TimeoutSeconds: intValue(item.Config, "timeout_seconds"), TLSInsecure: configBool(item.Config, "tls_insecure")}
	raw, configured, e := s.resourceSecret(ctx, item.ID)
	if e != nil {
		return e
	}
	if configured {
		var v map[string]any
		if e = json.Unmarshal(raw, &v); e != nil {
			return e
		}
		in.Username, in.Password = stringValue(v, "username"), stringValue(v, "password")
	}
	for _, t := range rmq.ListTools() {
		name := t.Name
		add(name, t.Description, rmqSchema(), func(c context.Context, _ map[string]any) (engine.ToolResult, error) {
			var out any
			var e error
			switch name {
			case "rabbitmq_health":
				out, e = rmq.Health(c, in)
			case "rabbitmq_overview":
				out, e = rmq.Overview(c, in)
			case "rabbitmq_nodes":
				out, e = rmq.Nodes(c, in)
			case "rabbitmq_queues":
				out, e = rmq.Queues(c, in)
			case "rabbitmq_exchanges":
				out, e = rmq.Exchanges(c, in)
			case "rabbitmq_connections":
				out, e = rmq.Connections(c, in)
			case "rabbitmq_channels":
				out, e = rmq.Channels(c, in)
			case "rabbitmq_consumers":
				out, e = rmq.Consumers(c, in)
			case "rabbitmq_vhosts":
				out, e = rmq.Vhosts(c, in)
			}
			if e != nil {
				return engine.ToolResult{}, rabbitMQError(name, e)
			}
			return engine.ToolResult{Output: out, Untrusted: true}, nil
		})
	}
	return nil
}
func rmqSchema() json.RawMessage {
	return json.RawMessage("{\"type\":\"object\",\"additionalProperties\":false}")
}
