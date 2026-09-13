package oracle

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	_ "github.com/sijms/go-ora/v2"
)

// ConnectionInput is transport-neutral and intentionally contains only the
// fields required to establish a read-only Oracle session.
type ConnectionInput struct {
	Host           string `json:"host,omitempty"`
	Port           int    `json:"port,omitempty"`
	ServiceName    string `json:"service_name,omitempty"`
	SID            string `json:"sid,omitempty"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	TLS            bool   `json:"tls,omitempty"`
}

type HealthOutput struct {
	Version       string `json:"version"`
	Database      string `json:"database"`
	Instance      string `json:"instance"`
	Status        string `json:"status"`
	UptimeSeconds int64  `json:"uptime_seconds"`
	LatencyMS     int64  `json:"latency_ms"`
}
type StatusOutput struct {
	Values map[string]any `json:"values"`
}
type PerformanceOutput struct {
	Values map[string]any `json:"values"`
}
type DatabaseInfoOutput struct {
	Values map[string]any `json:"values"`
}
type TableInfo struct {
	Schema          string `json:"schema"`
	Name            string `json:"name"`
	Tablespace      string `json:"tablespace"`
	Status          string `json:"status"`
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
	Name      string `json:"name"`
	Type      string `json:"type"`
	Nullable  string `json:"nullable"`
	Default   string `json:"default"`
	Comment   string `json:"comment"`
	Position  int    `json:"position"`
	Length    int64  `json:"length"`
	Precision int64  `json:"precision"`
	Scale     int64  `json:"scale"`
}
type TableColumnsOutput struct {
	Schema  string       `json:"schema"`
	Table   string       `json:"table"`
	Columns []ColumnInfo `json:"columns"`
}
type TableStructureOutput struct {
	Schema      string           `json:"schema"`
	Table       string           `json:"table"`
	Constraints []map[string]any `json:"constraints,omitempty"`
	Indexes     []map[string]any `json:"indexes,omitempty"`
}
type ToolInfo struct{ Name, Description string }

var tools = []ToolInfo{
	{Name: "oracle_health", Description: "Check Oracle connectivity and instance health."},
	{Name: "oracle_status", Description: "Read Oracle instance and session status."},
	{Name: "oracle_performance", Description: "Read Oracle performance counters and configuration parameters."},
	{Name: "oracle_database_info", Description: "Read Oracle database identity, role, mode and character set."},
	{Name: "oracle_tables", Description: "List user tables with statistics, tablespace and segment size."},
	{Name: "oracle_table_columns", Description: "Read columns, types, defaults and comments for one user table."},
	{Name: "oracle_table_structure", Description: "Read constraints and indexes for one user table."},
}

func ListTools() []ToolInfo { return append([]ToolInfo(nil), tools...) }
func InputSchema(extra map[string]any) map[string]any {
	p := map[string]any{"host": map[string]any{"type": "string"}, "port": map[string]any{"type": "integer", "minimum": 1, "maximum": 65535}, "service_name": map[string]any{"type": "string"}, "sid": map[string]any{"type": "string"}, "username": map[string]any{"type": "string"}, "password": map[string]any{"type": "string"}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": 300}, "tls": map[string]any{"type": "boolean"}}
	for k, v := range extra {
		p[k] = v
	}
	return map[string]any{"type": "object", "properties": p, "additionalProperties": false}
}

func dsn(in ConnectionInput) (string, error) {
	host, user := strings.TrimSpace(in.Host), strings.TrimSpace(in.Username)
	service, sid := strings.TrimSpace(in.ServiceName), strings.TrimSpace(in.SID)
	if host == "" || user == "" || in.Password == "" || (service == "" && sid == "") {
		return "", errors.New("host, username, password and service_name or sid are required")
	}
	if service != "" && sid != "" {
		return "", errors.New("service_name and sid are mutually exclusive")
	}
	port := in.Port
	if port <= 0 {
		port = 1521
	}
	if port > 65535 {
		return "", errors.New("port must be between 1 and 65535")
	}
	path := service
	if sid != "" {
		path = sid
	}
	options := ""
	if sid != "" {
		options = "?SID=" + url.QueryEscape(sid)
	}
	if in.TLS {
		if options == "" {
			options = "?"
		} else {
			options += "&"
		}
		options += "AUTH TYPE=TCPS"
	}
	return fmt.Sprintf("oracle://%s:%s@%s/%s%s", url.PathEscape(user), url.PathEscape(in.Password), net.JoinHostPort(host, strconv.Itoa(port)), url.PathEscape(path), options), nil
}

func open(ctx context.Context, in ConnectionInput) (*sql.DB, error) {
	d, err := dsn(in)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("oracle", d)
	if err != nil {
		return nil, err
	}
	if in.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(in.TimeoutSeconds)*time.Second)
		defer cancel()
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
func withDB[T any](ctx context.Context, in ConnectionInput, fn func(context.Context, *sql.DB) (T, error)) (T, error) {
	var zero T
	db, err := open(ctx, in)
	if err != nil {
		return zero, err
	}
	defer db.Close()
	return fn(ctx, db)
}

func queryMap(ctx context.Context, db *sql.DB, query string) (map[string]any, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]any{}
	for rows.Next() {
		var k string
		var v any
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		if b, ok := v.([]byte); ok {
			out[k] = string(b)
		} else {
			out[k] = v
		}
	}
	return out, rows.Err()
}

func Health(ctx context.Context, in ConnectionInput) (HealthOutput, error) {
	started := time.Now()
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (HealthOutput, error) {
		var o HealthOutput
		if err := db.QueryRowContext(ctx, "SELECT banner FROM v$version WHERE ROWNUM=1").Scan(&o.Version); err != nil {
			return o, err
		}
		_ = db.QueryRowContext(ctx, "SELECT SYS_CONTEXT('USERENV','DB_NAME') FROM dual").Scan(&o.Database)
		_ = db.QueryRowContext(ctx, "SELECT instance_name,status FROM v$instance").Scan(&o.Instance, &o.Status)
		_ = db.QueryRowContext(ctx, "SELECT (SYSDATE-startup_time)*86400 FROM v$instance").Scan(&o.UptimeSeconds)
		o.LatencyMS = time.Since(started).Milliseconds()
		return o, nil
	})
}
func Status(ctx context.Context, in ConnectionInput) (StatusOutput, error) {
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (StatusOutput, error) {
		v, e := queryMap(ctx, db, `SELECT 'instance_status',status FROM v$instance UNION ALL SELECT 'active_sessions',TO_CHAR(COUNT(*)) FROM v$session WHERE status='ACTIVE' UNION ALL SELECT 'transactions',TO_CHAR(COUNT(*)) FROM v$transaction`)
		return StatusOutput{Values: v}, e
	})
}
func Performance(ctx context.Context, in ConnectionInput) (PerformanceOutput, error) {
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (PerformanceOutput, error) {
		v, e := queryMap(ctx, db, `SELECT name,value FROM v$parameter WHERE name IN ('processes','sessions','open_cursors','pga_aggregate_target','sga_target','memory_target','parallel_max_servers','optimizer_mode') UNION ALL SELECT name,TO_CHAR(value) FROM v$sysstat WHERE name IN ('user commits','user rollbacks','physical reads','physical writes','session logical reads','execute count')`)
		return PerformanceOutput{Values: v}, e
	})
}
func DatabaseInfo(ctx context.Context, in ConnectionInput) (DatabaseInfoOutput, error) {
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (DatabaseInfoOutput, error) {
		v, e := queryMap(ctx, db, `SELECT 'db_name',name FROM v$database UNION ALL SELECT 'dbid',TO_CHAR(dbid) FROM v$database UNION ALL SELECT 'role',database_role FROM v$database UNION ALL SELECT 'open_mode',open_mode FROM v$database UNION ALL SELECT 'platform',platform_name FROM v$database UNION ALL SELECT 'instance',instance_name FROM v$instance UNION ALL SELECT 'version',version FROM v$instance`)
		return DatabaseInfoOutput{Values: v}, e
	})
}
func Tables(ctx context.Context, in ConnectionInput) (TablesOutput, error) {
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (TablesOutput, error) {
		rows, e := db.QueryContext(ctx, `SELECT t.owner,t.table_name,t.tablespace_name,t.status,COALESCE(s.num_rows,0),COALESCE(s.blocks,0)*8192 FROM all_tables t LEFT JOIN all_tab_statistics s ON s.owner=t.owner AND s.table_name=t.table_name AND s.partition_name IS NULL WHERE t.owner=SYS_CONTEXT('USERENV','CURRENT_SCHEMA') AND t.temporary='N' ORDER BY COALESCE(s.blocks,0) DESC FETCH FIRST 1000 ROWS ONLY`)
		if e != nil {
			return TablesOutput{}, e
		}
		defer rows.Close()
		out := TablesOutput{Tables: []TableInfo{}}
		for rows.Next() {
			var t TableInfo
			if e := rows.Scan(&t.Schema, &t.Name, &t.Tablespace, &t.Status, &t.Rows, &t.DiskBytes); e != nil {
				return out, e
			}
			out.Tables = append(out.Tables, t)
		}
		return out, rows.Err()
	})
}
func TableColumns(ctx context.Context, in ConnectionInput, input TableColumnsInput) (TableColumnsOutput, error) {
	input.Schema, input.Table = strings.TrimSpace(input.Schema), strings.TrimSpace(input.Table)
	if input.Schema == "" || input.Table == "" {
		return TableColumnsOutput{}, errors.New("schema and table are required")
	}
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (TableColumnsOutput, error) {
		rows, e := db.QueryContext(ctx, `SELECT column_id,column_name,data_type,nullable,data_default,data_length,data_precision,data_scale FROM all_tab_columns WHERE owner=:1 AND table_name=:2 ORDER BY column_id`, strings.ToUpper(input.Schema), strings.ToUpper(input.Table))
		if e != nil {
			return TableColumnsOutput{}, e
		}
		defer rows.Close()
		out := TableColumnsOutput{Schema: input.Schema, Table: input.Table, Columns: []ColumnInfo{}}
		for rows.Next() {
			var c ColumnInfo
			var def sql.NullString
			var precision, scale sql.NullInt64
			if e := rows.Scan(&c.Position, &c.Name, &c.Type, &c.Nullable, &def, &c.Length, &precision, &scale); e != nil {
				return out, e
			}
			c.Default = def.String
			c.Precision = precision.Int64
			c.Scale = scale.Int64
			out.Columns = append(out.Columns, c)
		}
		if e := rows.Err(); e != nil {
			return out, e
		}
		for i := range out.Columns {
			_ = db.QueryRowContext(ctx, `SELECT comments FROM all_col_comments WHERE owner=:1 AND table_name=:2 AND column_name=:3`, strings.ToUpper(input.Schema), strings.ToUpper(input.Table), out.Columns[i].Name).Scan(&out.Columns[i].Comment)
		}
		return out, nil
	})
}
func TableStructure(ctx context.Context, in ConnectionInput, input TableColumnsInput) (TableStructureOutput, error) {
	input.Schema, input.Table = strings.TrimSpace(input.Schema), strings.TrimSpace(input.Table)
	if input.Schema == "" || input.Table == "" {
		return TableStructureOutput{}, errors.New("schema and table are required")
	}
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (TableStructureOutput, error) {
		out := TableStructureOutput{Schema: input.Schema, Table: input.Table, Constraints: []map[string]any{}, Indexes: []map[string]any{}}
		rows, e := db.QueryContext(ctx, `SELECT constraint_name,constraint_type,status,validated FROM all_constraints WHERE owner=:1 AND table_name=:2 ORDER BY constraint_name`, strings.ToUpper(input.Schema), strings.ToUpper(input.Table))
		if e != nil {
			return out, e
		}
		defer rows.Close()
		for rows.Next() {
			var n, t, s, v string
			if e := rows.Scan(&n, &t, &s, &v); e != nil {
				return out, e
			}
			out.Constraints = append(out.Constraints, map[string]any{"name": n, "type": t, "status": s, "validated": v})
		}
		rows.Close()
		rows, e = db.QueryContext(ctx, `SELECT index_name,index_type,status,uniqueness FROM all_indexes WHERE owner=:1 AND table_name=:2 ORDER BY index_name`, strings.ToUpper(input.Schema), strings.ToUpper(input.Table))
		if e != nil {
			return out, e
		}
		defer rows.Close()
		for rows.Next() {
			var n, t, s, u string
			if e := rows.Scan(&n, &t, &s, &u); e != nil {
				return out, e
			}
			out.Indexes = append(out.Indexes, map[string]any{"name": n, "type": t, "status": s, "uniqueness": u})
		}
		return out, rows.Err()
	})
}
