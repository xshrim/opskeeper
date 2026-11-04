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
type lookupInput struct {
	pt.ConnectionInput
	Table string `json:"table"`
}
type slowQueriesInput struct {
	pt.ConnectionInput
	Limit      int     `json:"limit,omitempty"`
	MinTotalMS float64 `json:"min_total_ms,omitempty"`
}
type explainInput struct {
	pt.ConnectionInput
	Schema string `json:"schema,omitempty"`
	Query  string `json:"query"`
}

func columnsSchema() map[string]any {
	schema := pt.InputSchema(map[string]any{"schema": map[string]any{"type": "string"}, "table": map[string]any{"type": "string"}})
	schema["required"] = []string{"schema", "table"}
	return schema
}
func lookupSchema() map[string]any {
	schema := pt.InputSchema(map[string]any{"table": map[string]any{"type": "string"}})
	schema["required"] = []string{"table"}
	return schema
}
func slowQueriesSchema() map[string]any {
	return pt.InputSchema(map[string]any{"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 100}, "min_total_ms": map[string]any{"type": "number", "minimum": 0}})
}
func explainSchema() map[string]any {
	schema := pt.InputSchema(map[string]any{"schema": map[string]any{"type": "string"}, "query": map[string]any{"type": "string", "minLength": 1, "maxLength": 10000}})
	schema["required"] = []string{"query"}
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
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_table_structure", Description: "Read constraints and indexes for one user table.", InputSchema: columnsSchema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in columnsInput) (*mcp.CallToolResult, pt.TableStructureOutput, error) {
		o, e := pt.TableStructure(ctx, in.ConnectionInput, pt.TableStructureInput{Schema: in.Schema, Table: in.Table})
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_table_schema_lookup", Description: "Find visible PostgreSQL schemas containing a table.", InputSchema: lookupSchema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in lookupInput) (*mcp.CallToolResult, pt.SchemaLookupOutput, error) {
		o, e := pt.TableSchemaLookup(ctx, in.ConnectionInput, pt.SchemaLookupInput{Table: in.Table})
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_slow_queries", Description: "Rank normalized PostgreSQL queries by execution cost using pg_stat_statements.", InputSchema: slowQueriesSchema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in slowQueriesInput) (*mcp.CallToolResult, pt.SlowQueriesOutput, error) {
		o, e := pt.SlowQueries(ctx, in.ConnectionInput, pt.SlowQueriesInput{Limit: in.Limit, MinTotalMS: in.MinTotalMS})
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_explain", Description: "Read a safe, read-only PostgreSQL SELECT query plan.", InputSchema: explainSchema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in explainInput) (*mcp.CallToolResult, pt.ExplainOutput, error) {
		o, e := pt.Explain(ctx, in.ConnectionInput, pt.ExplainInput{Schema: in.Schema, Query: in.Query})
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_active_queries", Description: "Read bounded details about active PostgreSQL queries.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, pt.ActiveQueriesOutput, error) {
		o, e := pt.ActiveQueries(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_lock_waits", Description: "Read blocked PostgreSQL sessions and their blockers.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, pt.LockWaitsOutput, error) {
		o, e := pt.LockWaits(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_vacuum_table_stats", Description: "Read per-table PostgreSQL vacuum and dead-tuple statistics.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, pt.VacuumTableStatsOutput, error) {
		o, e := pt.VacuumTableStats(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_connection_stats", Description: "Read PostgreSQL connection counts and utilization.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, pt.ConnectionStatsOutput, error) {
		o, e := pt.ConnectionStats(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_connection_groups", Description: "Group PostgreSQL connections by state, user and client.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, pt.ConnectionGroupsOutput, error) {
		o, e := pt.ConnectionGroups(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "postgresql_idle_sessions", Description: "Read the oldest bounded PostgreSQL idle sessions.", InputSchema: schema()}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, pt.IdleSessionsOutput, error) {
		o, e := pt.IdleSessions(ctx, in.ConnectionInput)
		return nil, o, e
	})
}
