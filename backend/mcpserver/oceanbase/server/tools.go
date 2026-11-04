package server

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	ot "opskeeper/backend/tool/oceanbase"
)

type input struct{ ot.ConnectionInput }
type tableInput struct {
	ot.ConnectionInput
	Schema string `json:"schema"`
	Table  string `json:"table"`
}
type slowQueriesInput struct {
	ot.ConnectionInput
	Limit      int     `json:"limit,omitempty"`
	MinTotalMS float64 `json:"min_total_ms,omitempty"`
}

func schema() map[string]any { return ot.InputSchema(nil) }
func tableSchema() map[string]any {
	s := ot.InputSchema(map[string]any{"schema": map[string]any{"type": "string"}, "table": map[string]any{"type": "string"}})
	s["required"] = []string{"schema", "table"}
	return s
}
func RegisterTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "oceanbase_health", Description: "Check OceanBase connectivity and server health.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, ot.HealthOutput, error) {
		o, e := ot.Health(ctx, in.ConnectionInput)
		return nil, o, e
	})
	for _, d := range []struct {
		name, desc string
		fn         func(context.Context, ot.ConnectionInput) (ot.ValuesOutput, error)
	}{{"oceanbase_status", "Read OceanBase server status counters.", ot.Status}, {"oceanbase_performance", "Read OceanBase performance counters and configuration.", ot.Performance}, {"oceanbase_database_info", "Read OceanBase database identity and configuration.", ot.DatabaseInfo}} {
		d := d
		mcp.AddTool(s, &mcp.Tool{Name: d.name, Description: d.desc, InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, ot.ValuesOutput, error) {
			o, e := d.fn(ctx, in.ConnectionInput)
			return nil, o, e
		})
	}
	mcp.AddTool(s, &mcp.Tool{Name: "oceanbase_tables", Description: "List OceanBase user tables with row counts and size.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, ot.TablesOutput, error) {
		o, e := ot.Tables(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "oceanbase_table_columns", Description: "Read columns, types, defaults and comments for one table.", InputSchema: tableSchema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in tableInput) (*mcp.CallToolResult, ot.TableColumnsOutput, error) {
		o, e := ot.TableColumns(ctx, in.ConnectionInput, ot.TableColumnsInput{Schema: in.Schema, Table: in.Table})
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "oceanbase_table_structure", Description: "Read indexes and constraints for one table.", InputSchema: tableSchema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in tableInput) (*mcp.CallToolResult, ot.TableStructureOutput, error) {
		o, e := ot.TableStructure(ctx, in.ConnectionInput, ot.TableColumnsInput{Schema: in.Schema, Table: in.Table})
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "oceanbase_slow_queries", Description: "Rank non-internal SQL Audit statements from the last 30 minutes by cumulative execution time; threshold is per SQL ID and availability depends on tenant mode, version and privileges.", InputSchema: ot.InputSchema(map[string]any{"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 100}, "min_total_ms": map[string]any{"type": "number", "minimum": 0}})}, func(ctx context.Context, _ *mcp.CallToolRequest, in slowQueriesInput) (*mcp.CallToolResult, ot.SlowQueriesOutput, error) {
		o, e := ot.SlowQueries(ctx, in.ConnectionInput, ot.SlowQueriesInput{Limit: in.Limit, MinTotalMS: in.MinTotalMS})
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "oceanbase_active_queries", Description: "Read active sessions from the OceanBase tenant process list; availability depends on tenant mode, version and privileges.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, ot.ActiveQueriesOutput, error) {
		o, e := ot.ActiveQueries(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "oceanbase_lock_waits", Description: "Read OceanBase transaction lock waits and blocker sessions when supported by the tenant mode, version and privileges.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, ot.LockWaitsOutput, error) {
		o, e := ot.LockWaits(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "oceanbase_connection_stats", Description: "Read OceanBase tenant session counts; maximum connection capacity may not be exposed.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, ot.ConnectionStatsOutput, error) {
		o, e := ot.ConnectionStats(ctx, in.ConnectionInput)
		return nil, o, e
	})
}
