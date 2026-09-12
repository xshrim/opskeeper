package postgresql

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ConnectionInput is transport-neutral. Direct resources fill it from their
// config and credential; MCP resources receive the same fields server-side.
type ConnectionInput struct {
	Host           string `json:"host,omitempty"`
	Port           int    `json:"port,omitempty"`
	Database       string `json:"database,omitempty"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

type HealthOutput struct {
	ServerVersion string `json:"server_version"`
	Database      string `json:"database"`
	LatencyMS     int64  `json:"latency_ms"`
}
type SessionsOutput struct {
	Active int `json:"active_sessions"`
}
type LongRunningQueriesOutput struct {
	Count int `json:"long_running_queries"`
}
type LocksOutput struct {
	Waiting int `json:"waiting_locks"`
}
type ReplicationOutput struct {
	Replicas int `json:"replica_count"`
}
type CapacityOutput struct {
	DatabaseSizeBytes int64 `json:"database_size_bytes"`
}
type TableInfo struct {
	Schema          string `json:"schema"`
	Name            string `json:"name"`
	Rows            int64  `json:"rows"`
	DiskBytes       int64  `json:"disk_bytes"`
	SequentialScans int64  `json:"sequential_scans"`
	IndexScans      int64  `json:"index_scans"`
	Inserts         int64  `json:"inserts"`
	Updates         int64  `json:"updates"`
	Deletes         int64  `json:"deletes"`
}
type TablesOutput struct {
	Tables []TableInfo `json:"tables"`
}
type TableColumnsInput struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
}
type ColumnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable string `json:"nullable"`
	Default  string `json:"default"`
	Comment  string `json:"comment"`
}
type TableColumnsOutput struct {
	Schema  string       `json:"schema"`
	Table   string       `json:"table"`
	Columns []ColumnInfo `json:"columns"`
}
type PerformanceOutput struct {
	Values map[string]any `json:"values"`
}
type VacuumOutput struct {
	Values map[string]any `json:"values"`
}
type ExtensionInfo struct {
	Name, Version string
	Installed     bool `json:"installed"`
}
type ExtensionsOutput struct {
	Extensions []ExtensionInfo `json:"extensions"`
}
type DatabaseInfoOutput struct {
	Values map[string]any `json:"values"`
}

type ToolInfo struct{ Name, Description string }

var tools = []ToolInfo{
	{Name: "postgresql_health", Description: "Check PostgreSQL connectivity and server health."},
	{Name: "postgresql_sessions", Description: "Read the number of active PostgreSQL sessions."},
	{Name: "postgresql_long_running_queries", Description: "Read active PostgreSQL queries running longer than five seconds."},
	{Name: "postgresql_locks", Description: "Read PostgreSQL locks waiting to be granted."},
	{Name: "postgresql_replication", Description: "Read the number of connected PostgreSQL replicas."},
	{Name: "postgresql_capacity", Description: "Read the current database size."},
	{Name: "postgresql_tables", Description: "List user tables with size, row and scan/write statistics."},
	{Name: "postgresql_performance", Description: "Read PostgreSQL performance counters and configuration parameters."},
	{Name: "postgresql_vacuum", Description: "Read PostgreSQL VACUUM and autovacuum configuration."},
	{Name: "postgresql_extensions", Description: "List installed PostgreSQL extensions and versions."},
	{Name: "postgresql_database_info", Description: "Read PostgreSQL database identity and general information."},
	{Name: "postgresql_table_columns", Description: "Read columns, types, defaults and comments for one user table."},
}

func ListTools() []ToolInfo { return append([]ToolInfo(nil), tools...) }

func InputSchema(extra map[string]any) map[string]any {
	properties := map[string]any{
		"host":            map[string]any{"type": "string", "description": "PostgreSQL host."},
		"port":            map[string]any{"type": "integer", "minimum": 1, "maximum": 65535},
		"database":        map[string]any{"type": "string"},
		"username":        map[string]any{"type": "string"},
		"password":        map[string]any{"type": "string"},
		"timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": 300},
	}
	for key, value := range extra {
		properties[key] = value
	}
	return map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
}

func open(ctx context.Context, input ConnectionInput) (*pgx.Conn, error) {
	host := strings.TrimSpace(input.Host)
	database := strings.TrimSpace(input.Database)
	username := strings.TrimSpace(input.Username)
	if host == "" || database == "" || username == "" || input.Password == "" {
		return nil, errors.New("host, database, username and password are required")
	}
	port := input.Port
	if port <= 0 {
		port = 5432
	}
	if port > 65535 {
		return nil, errors.New("port must be between 1 and 65535")
	}
	connectionURL := (&url.URL{Scheme: "postgres", User: url.UserPassword(username, input.Password), Host: net.JoinHostPort(host, strconv.Itoa(port)), Path: database, RawQuery: "sslmode=prefer"}).String()
	config, err := pgx.ParseConfig(connectionURL)
	if err != nil {
		return nil, fmt.Errorf("parse PostgreSQL connection: %w", err)
	}
	config.RuntimeParams["default_transaction_read_only"] = "on"
	config.RuntimeParams["application_name"] = "opskeeper-diagnostic"
	if input.TimeoutSeconds > 0 {
		config.RuntimeParams["statement_timeout"] = strconv.Itoa(input.TimeoutSeconds * 1000)
	}
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func withConn[T any](ctx context.Context, input ConnectionInput, fn func(context.Context, *pgx.Conn) (T, error)) (T, error) {
	var zero T
	conn, err := open(ctx, input)
	if err != nil {
		return zero, err
	}
	defer conn.Close(ctx)
	if err := conn.Ping(ctx); err != nil {
		return zero, err
	}
	return fn(ctx, conn)
}

func Health(ctx context.Context, input ConnectionInput) (HealthOutput, error) {
	started := time.Now()
	return withConn(ctx, input, func(ctx context.Context, conn *pgx.Conn) (HealthOutput, error) {
		var out HealthOutput
		if err := conn.QueryRow(ctx, "SHOW server_version").Scan(&out.ServerVersion); err != nil {
			return out, err
		}
		if err := conn.QueryRow(ctx, "SELECT current_database()").Scan(&out.Database); err != nil {
			return out, err
		}
		out.LatencyMS = time.Since(started).Milliseconds()
		return out, nil
	})
}
func Sessions(ctx context.Context, input ConnectionInput) (SessionsOutput, error) {
	return withConn(ctx, input, func(ctx context.Context, conn *pgx.Conn) (SessionsOutput, error) {
		var out SessionsOutput
		err := conn.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE state = 'active'").Scan(&out.Active)
		return out, err
	})
}
func LongRunningQueries(ctx context.Context, input ConnectionInput) (LongRunningQueriesOutput, error) {
	return withConn(ctx, input, func(ctx context.Context, conn *pgx.Conn) (LongRunningQueriesOutput, error) {
		var out LongRunningQueriesOutput
		err := conn.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE state = 'active' AND pid <> pg_backend_pid() AND query_start < clock_timestamp() - interval '5 seconds'").Scan(&out.Count)
		return out, err
	})
}
func Locks(ctx context.Context, input ConnectionInput) (LocksOutput, error) {
	return withConn(ctx, input, func(ctx context.Context, conn *pgx.Conn) (LocksOutput, error) {
		var out LocksOutput
		err := conn.QueryRow(ctx, "SELECT count(*) FROM pg_locks WHERE NOT granted").Scan(&out.Waiting)
		return out, err
	})
}
func Replication(ctx context.Context, input ConnectionInput) (ReplicationOutput, error) {
	return withConn(ctx, input, func(ctx context.Context, conn *pgx.Conn) (ReplicationOutput, error) {
		var out ReplicationOutput
		err := conn.QueryRow(ctx, "SELECT count(*) FROM pg_stat_replication").Scan(&out.Replicas)
		return out, err
	})
}
func Capacity(ctx context.Context, input ConnectionInput) (CapacityOutput, error) {
	return withConn(ctx, input, func(ctx context.Context, conn *pgx.Conn) (CapacityOutput, error) {
		var out CapacityOutput
		err := conn.QueryRow(ctx, "SELECT pg_database_size(current_database())").Scan(&out.DatabaseSizeBytes)
		return out, err
	})
}

func Tables(ctx context.Context, input ConnectionInput) (TablesOutput, error) {
	return withConn(ctx, input, func(ctx context.Context, conn *pgx.Conn) (TablesOutput, error) {
		rows, err := conn.Query(ctx, `SELECT n.nspname, c.relname, COALESCE(s.n_live_tup,0), pg_total_relation_size(c.oid), COALESCE(s.seq_scan,0), COALESCE(s.idx_scan,0), COALESCE(s.n_tup_ins,0), COALESCE(s.n_tup_upd,0), COALESCE(s.n_tup_del,0) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace LEFT JOIN pg_stat_user_tables s ON s.relid=c.oid WHERE c.relkind IN ('r','p') AND n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_toast' ORDER BY pg_total_relation_size(c.oid) DESC LIMIT 1000`)
		if err != nil {
			return TablesOutput{}, err
		}
		defer rows.Close()
		out := TablesOutput{Tables: []TableInfo{}}
		for rows.Next() {
			var t TableInfo
			if err := rows.Scan(&t.Schema, &t.Name, &t.Rows, &t.DiskBytes, &t.SequentialScans, &t.IndexScans, &t.Inserts, &t.Updates, &t.Deletes); err != nil {
				return out, err
			}
			out.Tables = append(out.Tables, t)
		}
		return out, rows.Err()
	})
}

func TableColumns(ctx context.Context, connection ConnectionInput, input TableColumnsInput) (TableColumnsOutput, error) {
	input.Schema, input.Table = strings.TrimSpace(input.Schema), strings.TrimSpace(input.Table)
	if input.Schema == "" || input.Table == "" {
		return TableColumnsOutput{}, errors.New("schema and table are required")
	}
	return withConn(ctx, connection, func(ctx context.Context, conn *pgx.Conn) (TableColumnsOutput, error) {
		rows, err := conn.Query(ctx, `SELECT c.column_name,c.data_type,CASE WHEN c.is_nullable='YES' THEN 'YES' ELSE 'NO' END,COALESCE(c.column_default,''),COALESCE(d.description,'') FROM information_schema.columns c LEFT JOIN pg_catalog.pg_class t ON t.relname=c.table_name LEFT JOIN pg_catalog.pg_namespace n ON n.oid=t.relnamespace AND n.nspname=c.table_schema LEFT JOIN pg_catalog.pg_description d ON d.objoid=t.oid AND d.objsubid=c.ordinal_position WHERE c.table_schema=$1 AND c.table_name=$2 AND c.table_schema NOT IN ('pg_catalog','information_schema') ORDER BY c.ordinal_position`, input.Schema, input.Table)
		if err != nil {
			return TableColumnsOutput{}, err
		}
		defer rows.Close()
		out := TableColumnsOutput{Schema: input.Schema, Table: input.Table, Columns: []ColumnInfo{}}
		for rows.Next() {
			var c ColumnInfo
			if err := rows.Scan(&c.Name, &c.Type, &c.Nullable, &c.Default, &c.Comment); err != nil {
				return out, err
			}
			out.Columns = append(out.Columns, c)
		}
		if err := rows.Err(); err != nil {
			return out, err
		}
		if len(out.Columns) == 0 {
			return out, fmt.Errorf("user table %s.%s not found", input.Schema, input.Table)
		}
		return out, nil
	})
}

func querySettings(ctx context.Context, input ConnectionInput, pattern string) (map[string]any, error) {
	return withConn(ctx, input, func(ctx context.Context, conn *pgx.Conn) (map[string]any, error) {
		rows, err := conn.Query(ctx, "SELECT name, setting, COALESCE(unit, ''), COALESCE(short_desc, '') FROM pg_settings WHERE name ~ $1 ORDER BY name LIMIT 200", pattern)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		values := map[string]any{}
		for rows.Next() {
			var n, s, u, d string
			if err := rows.Scan(&n, &s, &u, &d); err != nil {
				return nil, err
			}
			values[n] = map[string]string{"setting": s, "unit": u, "description": d}
		}
		return values, rows.Err()
	})
}
func Performance(ctx context.Context, input ConnectionInput) (PerformanceOutput, error) {
	return withConn(ctx, input, func(ctx context.Context, conn *pgx.Conn) (PerformanceOutput, error) {
		values := map[string]any{}
		rows, err := conn.Query(ctx, "SELECT name, setting, COALESCE(unit, ''), COALESCE(short_desc, '') FROM pg_settings WHERE name ~ $1 ORDER BY name LIMIT 200", `^(shared_buffers|work_mem|max_connections|effective_cache_size|checkpoint_|bgwriter_|wal_|track_|log_)`)
		if err != nil {
			return PerformanceOutput{}, err
		}
		for rows.Next() {
			var n, s, u, d string
			if err := rows.Scan(&n, &s, &u, &d); err != nil {
				rows.Close()
				return PerformanceOutput{}, err
			}
			values[n] = map[string]string{"setting": s, "unit": u, "description": d}
		}
		rows.Close()
		var num int
		var commit, rollback, read, hit, returned, fetched, inserted, updated, deleted int64
		if err := conn.QueryRow(ctx, "SELECT numbackends,xact_commit,xact_rollback,blks_read,blks_hit,tup_returned,tup_fetched,tup_inserted,tup_updated,tup_deleted FROM pg_stat_database WHERE datname=current_database()").Scan(&num, &commit, &rollback, &read, &hit, &returned, &fetched, &inserted, &updated, &deleted); err == nil {
			values["database_stats"] = map[string]any{"numbackends": num, "xact_commit": commit, "xact_rollback": rollback, "blocks_read": read, "blocks_hit": hit, "tuples_returned": returned, "tuples_fetched": fetched, "tuples_inserted": inserted, "tuples_updated": updated, "tuples_deleted": deleted}
		}
		return PerformanceOutput{Values: values}, nil
	})
}
func Vacuum(ctx context.Context, input ConnectionInput) (VacuumOutput, error) {
	values, err := querySettings(ctx, input, `^(autovacuum|vacuum_cost|vacuum_freeze|maintenance_work_mem)`)
	return VacuumOutput{Values: values}, err
}
func Extensions(ctx context.Context, input ConnectionInput) (ExtensionsOutput, error) {
	return withConn(ctx, input, func(ctx context.Context, conn *pgx.Conn) (ExtensionsOutput, error) {
		rows, err := conn.Query(ctx, "SELECT e.extname,e.extversion,true FROM pg_extension e ORDER BY e.extname")
		if err != nil {
			return ExtensionsOutput{}, err
		}
		defer rows.Close()
		out := ExtensionsOutput{Extensions: []ExtensionInfo{}}
		for rows.Next() {
			var e ExtensionInfo
			if err := rows.Scan(&e.Name, &e.Version, &e.Installed); err != nil {
				return out, err
			}
			out.Extensions = append(out.Extensions, e)
		}
		return out, rows.Err()
	})
}
func DatabaseInfo(ctx context.Context, input ConnectionInput) (DatabaseInfoOutput, error) {
	return withConn(ctx, input, func(ctx context.Context, conn *pgx.Conn) (DatabaseInfoOutput, error) {
		var out DatabaseInfoOutput
		out.Values = map[string]any{}
		var v, enc, coll string
		var size int64
		if err := conn.QueryRow(ctx, "SELECT current_database(),pg_encoding_to_char(encoding),datcollate,pg_database_size(current_database()) FROM pg_database WHERE datname=current_database()").Scan(&v, &enc, &coll, &size); err != nil {
			return out, err
		}
		out.Values["database"] = v
		out.Values["encoding"] = enc
		out.Values["collation"] = coll
		out.Values["size_bytes"] = size
		return out, nil
	})
}
