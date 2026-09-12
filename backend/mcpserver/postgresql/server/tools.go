package server

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	pt "opskeeper/backend/tool/postgresql"
)

type input struct{ pt.ConnectionInput }
type columnsInput struct {
	pt.ConnectionInput
	Schema string `json:"schema"`
	Table  string `json:"table"`
}

func columnsSchema() map[string]any {
	schema := pt.InputSchema(map[string]any{"schema": map[string]any{"type": "string"}, "table": map[string]any{"type": "string"}})
	schema["required"] = []string{"schema", "table"}
	return schema
}

func schema() map[string]any { return pt.InputSchema(nil) }
func RegisterTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_health", Description: "Check PostgreSQL connectivity and server health.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, pt.HealthOutput, error) {
		o, e := pt.Health(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_sessions", Description: "Read the number of active PostgreSQL sessions.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, pt.SessionsOutput, error) {
		o, e := pt.Sessions(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_long_running_queries", Description: "Read active PostgreSQL queries running longer than five seconds.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, pt.LongRunningQueriesOutput, error) {
		o, e := pt.LongRunningQueries(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_locks", Description: "Read PostgreSQL locks waiting to be granted.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, pt.LocksOutput, error) {
		o, e := pt.Locks(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_replication", Description: "Read the number of connected PostgreSQL replicas.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, pt.ReplicationOutput, error) {
		o, e := pt.Replication(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_capacity", Description: "Read the current database size.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, pt.CapacityOutput, error) {
		o, e := pt.Capacity(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_tables", Description: "List user tables with size, row and scan/write statistics.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, pt.TablesOutput, error) {
		o, e := pt.Tables(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_performance", Description: "Read PostgreSQL performance counters and configuration parameters.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, pt.PerformanceOutput, error) {
		o, e := pt.Performance(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_vacuum", Description: "Read PostgreSQL VACUUM and autovacuum configuration.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, pt.VacuumOutput, error) {
		o, e := pt.Vacuum(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_extensions", Description: "List installed PostgreSQL extensions and versions.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, pt.ExtensionsOutput, error) {
		o, e := pt.Extensions(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_database_info", Description: "Read PostgreSQL database identity and general information.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, pt.DatabaseInfoOutput, error) {
		o, e := pt.DatabaseInfo(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_table_columns", Description: "Read columns, types, defaults and comments for one user table.", InputSchema: columnsSchema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in columnsInput) (*mcp.CallToolResult, pt.TableColumnsOutput, error) {
		o, e := pt.TableColumns(ctx, in.ConnectionInput, pt.TableColumnsInput{Schema: in.Schema, Table: in.Table})
		return nil, o, e
	})
}
