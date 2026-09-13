package server

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	kt "opskeeper/backend/tool/kafka"
)

type input struct{ kt.ConnectionInput }
type topicInput struct {
	kt.ConnectionInput
	Topic string `json:"topic"`
}
type lagInput struct {
	kt.ConnectionInput
	Limit int `json:"limit,omitempty"`
}

func schema(extra map[string]any) map[string]any { return kt.InputSchema(extra) }
func RegisterTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "kafka_health", Description: "Check Kafka cluster health and exporter-style summary.", InputSchema: schema(nil)}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, kt.HealthOutput, error) {
		o, e := kt.Health(c, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "kafka_brokers", Description: "List Kafka brokers.", InputSchema: schema(nil)}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, kt.BrokersOutput, error) {
		o, e := kt.Brokers(c, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "kafka_topics", Description: "List bounded Kafka topic metrics.", InputSchema: schema(nil)}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, kt.TopicsOutput, error) {
		o, e := kt.Topics(c, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "kafka_consumer_groups", Description: "List Kafka consumer groups.", InputSchema: schema(nil)}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, kt.ConsumerGroupsOutput, error) {
		o, e := kt.ConsumerGroups(c, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "kafka_consumer_lag", Description: "Read bounded Kafka consumer lag.", InputSchema: schema(map[string]any{"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 1000}})}, func(c context.Context, _ *mcp.CallToolRequest, in lagInput) (*mcp.CallToolResult, kt.ConsumerLagOutput, error) {
		o, e := kt.ConsumerLag(c, in.ConnectionInput, kt.ConsumerLagInput{Limit: in.Limit})
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "kafka_cluster_info", Description: "Read Kafka controller and broker metadata.", InputSchema: schema(nil)}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, kt.ClusterInfoOutput, error) {
		o, e := kt.ClusterInfo(c, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "kafka_topic_partitions", Description: "Read partitions for a specific Kafka topic.", InputSchema: schema(map[string]any{"topic": map[string]any{"type": "string", "minLength": 1}})}, func(c context.Context, _ *mcp.CallToolRequest, in topicInput) (*mcp.CallToolResult, kt.TopicPartitionsOutput, error) {
		o, e := kt.TopicPartitions(c, in.ConnectionInput, kt.TopicPartitionsInput{Topic: in.Topic})
		return nil, o, e
	})
}
