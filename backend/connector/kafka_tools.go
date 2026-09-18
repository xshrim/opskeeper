package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opskeeper/backend/aiengine"
	kt "opskeeper/backend/tool/kafka"
)

func (s *Service) resolveKafkaTools(ctx context.Context, item aiengine.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (aiengine.ToolResult, error))) error {
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "agent") {
		return fmt.Errorf("Kafka agent resources must use the MCP provider")
	}
	in := kt.ConnectionInput{Brokers: brokerAddresses(item.Config["brokers"]), TLS: configBool(item.Config, "tls"), TLSServerName: stringValue(item.Config, "tls_server_name"), TimeoutSeconds: intValue(item.Config, "timeout_seconds")}
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
	for _, t := range kt.ListTools() {
		name, desc := t.Name, t.Description
		add(name, desc, kafkaSchema(name), func(rc context.Context, args map[string]any) (aiengine.ToolResult, error) {
			var out any
			var e error
			switch name {
			case "kafka_health":
				out, e = kt.Health(rc, in)
			case "kafka_brokers":
				out, e = kt.Brokers(rc, in)
			case "kafka_topics":
				out, e = kt.Topics(rc, in)
			case "kafka_consumer_groups":
				out, e = kt.ConsumerGroups(rc, in)
			case "kafka_cluster_info":
				out, e = kt.ClusterInfo(rc, in)
			case "kafka_consumer_lag":
				var a kt.ConsumerLagInput
				b, _ := json.Marshal(args)
				_ = json.Unmarshal(b, &a)
				out, e = kt.ConsumerLag(rc, in, a)
			case "kafka_topic_partitions":
				var a kt.TopicPartitionsInput
				b, _ := json.Marshal(args)
				_ = json.Unmarshal(b, &a)
				out, e = kt.TopicPartitions(rc, in, a)
			}
			if e != nil {
				return aiengine.ToolResult{}, fmt.Errorf("%s: %w", name, e)
			}
			return aiengine.ToolResult{Output: out, Untrusted: true}, nil
		})
	}
	return nil
}
func kafkaSchema(name string) json.RawMessage {
	if name == "kafka_topic_partitions" {
		return json.RawMessage(`{"type":"object","required":["topic"],"properties":{"topic":{"type":"string","minLength":1}},"additionalProperties":false}`)
	}
	if name == "kafka_consumer_lag" {
		return json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer","minimum":1,"maximum":1000}},"additionalProperties":false}`)
	}
	return json.RawMessage(`{"type":"object","additionalProperties":false}`)
}
