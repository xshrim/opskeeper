package server

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	rt "opskeeper/backend/tool/redis"
)

type input struct{ rt.ConnectionInput }

func schema() map[string]any { return rt.InputSchema(nil) }
func RegisterTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "redis_health", Description: "Check Redis connectivity and server health.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, rt.HealthOutput, error) {
		o, e := rt.Health(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "redis_memory", Description: "Read Redis memory statistics.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, rt.ValuesOutput, error) {
		o, e := rt.Memory(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "redis_clients", Description: "Read Redis client statistics.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, rt.ValuesOutput, error) {
		o, e := rt.Clients(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "redis_replication", Description: "Read Redis replication status.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, rt.ValuesOutput, error) {
		o, e := rt.Replication(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "redis_slowlog", Description: "Read the latest bounded Redis slowlog entries.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, rt.SlowlogOutput, error) {
		o, e := rt.Slowlog(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "redis_database_info", Description: "Read Redis keyspace and selected database information.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, rt.DatabaseInfoOutput, error) {
		o, e := rt.DatabaseInfo(ctx, in.ConnectionInput)
		return nil, o, e
	})
}
