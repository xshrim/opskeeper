package server

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"opskeeper/backend/tool/oracle"
)

type input struct{ oracle.ConnectionInput }
type tableInput struct {
	oracle.ConnectionInput
	Schema string `json:"schema"`
	Table  string `json:"table"`
}

func schema() map[string]any { return oracle.InputSchema(nil) }
func tableSchema() map[string]any {
	s := oracle.InputSchema(map[string]any{"schema": map[string]any{"type": "string"}, "table": map[string]any{"type": "string"}})
	s["required"] = []string{"schema", "table"}
	return s
}
func RegisterTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "oracle_health", Description: "Check Oracle connectivity and instance health.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, oracle.HealthOutput, error) {
		o, e := oracle.Health(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "oracle_status", Description: "Read Oracle instance and session status.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, oracle.StatusOutput, error) {
		o, e := oracle.Status(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "oracle_performance", Description: "Read Oracle performance counters and configuration parameters.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, oracle.PerformanceOutput, error) {
		o, e := oracle.Performance(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "oracle_database_info", Description: "Read Oracle database identity, role, mode and character set.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, oracle.DatabaseInfoOutput, error) {
		o, e := oracle.DatabaseInfo(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "oracle_tables", Description: "List user tables with statistics, tablespace and segment size.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, oracle.TablesOutput, error) {
		o, e := oracle.Tables(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "oracle_table_columns", Description: "Read columns, types, defaults and comments for one user table.", InputSchema: tableSchema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in tableInput) (*mcp.CallToolResult, oracle.TableColumnsOutput, error) {
		o, e := oracle.TableColumns(ctx, in.ConnectionInput, oracle.TableColumnsInput{Schema: in.Schema, Table: in.Table})
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "oracle_table_structure", Description: "Read constraints and indexes for one user table.", InputSchema: tableSchema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in tableInput) (*mcp.CallToolResult, oracle.TableStructureOutput, error) {
		o, e := oracle.TableStructure(ctx, in.ConnectionInput, oracle.TableColumnsInput{Schema: in.Schema, Table: in.Table})
		return nil, o, e
	})
}
