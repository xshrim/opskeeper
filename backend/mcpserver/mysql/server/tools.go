package server

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	mt "opskeeper/backend/tool/mysql"
)

type input struct{ mt.ConnectionInput }
type columnsInput struct {
	mt.ConnectionInput
	Schema string `json:"schema"`
	Table  string `json:"table"`
}
type slowQueriesInput struct {
	mt.ConnectionInput
	Limit      int     `json:"limit,omitempty"`
	MinTotalMS float64 `json:"min_total_ms,omitempty"`
}

func schema() map[string]any { return mt.InputSchema(nil) }
func columnsSchema() map[string]any {
	s := mt.InputSchema(map[string]any{"schema": map[string]any{"type": "string"}, "table": map[string]any{"type": "string"}})
	s["required"] = []string{"schema", "table"}
	return s
}
func RegisterTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "mysql_health", Description: "Check MySQL connectivity and server health.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, mt.HealthOutput, error) {
		o, e := mt.Health(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "mysql_status", Description: "Read MySQL server status counters.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, mt.StatusOutput, error) {
		o, e := mt.Status(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "mysql_performance", Description: "Read MySQL performance counters and configuration parameters.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, mt.PerformanceOutput, error) {
		o, e := mt.Performance(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "mysql_tables", Description: "List user tables with row counts, size and metadata.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, mt.TablesOutput, error) {
		o, e := mt.Tables(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "mysql_table_columns", Description: "Read columns, types, defaults and comments for one user table.", InputSchema: columnsSchema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in columnsInput) (*mcp.CallToolResult, mt.TableColumnsOutput, error) {
		o, e := mt.TableColumns(ctx, in.ConnectionInput, mt.TableColumnsInput{Schema: in.Schema, Table: in.Table})
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "mysql_table_structure", Description: "Read the CREATE TABLE definition and indexes for one user table.", InputSchema: columnsSchema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in columnsInput) (*mcp.CallToolResult, mt.TableStructureOutput, error) {
		o, e := mt.TableStructure(ctx, in.ConnectionInput, mt.TableColumnsInput{Schema: in.Schema, Table: in.Table})
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "mysql_database_info", Description: "Read MySQL database identity and general information.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, mt.DatabaseInfoOutput, error) {
		o, e := mt.DatabaseInfo(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "mysql_slow_queries", Description: "Rank normalized MySQL statements by execution cost.", InputSchema: mt.InputSchema(map[string]any{"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 100}, "min_total_ms": map[string]any{"type": "number", "minimum": 0}})}, func(ctx context.Context, _ *mcp.CallToolRequest, in slowQueriesInput) (*mcp.CallToolResult, mt.SlowQueriesOutput, error) {
		o, e := mt.SlowQueries(ctx, in.ConnectionInput, mt.SlowQueriesInput{Limit: in.Limit, MinTotalMS: in.MinTotalMS})
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "mysql_active_queries", Description: "Read bounded details about active MySQL statements.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, mt.ActiveQueriesOutput, error) {
		o, e := mt.ActiveQueries(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "mysql_lock_waits", Description: "Read waiting and blocking MySQL transactions.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, mt.LockWaitsOutput, error) {
		o, e := mt.LockWaits(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "mysql_connection_stats", Description: "Read MySQL connection counts and utilization.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, mt.ConnectionStatsOutput, error) {
		o, e := mt.ConnectionStats(ctx, in.ConnectionInput)
		return nil, o, e
	})
}
