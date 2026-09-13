package mysql

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	driver "github.com/go-sql-driver/mysql"
)

type ConnectionInput struct {
	Host           string `json:"host,omitempty"`
	Port           int    `json:"port,omitempty"`
	Database       string `json:"database,omitempty"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}
type HealthOutput struct {
	Version          string `json:"version"`
	Database         string `json:"database"`
	UptimeSeconds    int64  `json:"uptime_seconds"`
	ThreadsConnected int64  `json:"threads_connected"`
	LatencyMS        int64  `json:"latency_ms"`
}
type StatusOutput struct {
	Values map[string]any `json:"values"`
}
type PerformanceOutput struct {
	Values map[string]any `json:"values"`
}
type TableInfo struct {
	Schema    string `json:"schema"`
	Name      string `json:"name"`
	Rows      int64  `json:"rows"`
	DiskBytes int64  `json:"disk_bytes"`
	Engine    string `json:"engine"`
	Collation string `json:"collation"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
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
	Position int    `json:"position"`
}
type TableColumnsOutput struct {
	Schema  string       `json:"schema"`
	Table   string       `json:"table"`
	Columns []ColumnInfo `json:"columns"`
}
type TableStructureOutput struct {
	Schema    string           `json:"schema"`
	Table     string           `json:"table"`
	CreateSQL string           `json:"create_sql"`
	Indexes   []map[string]any `json:"indexes"`
}
type DatabaseInfoOutput struct {
	Values map[string]any `json:"values"`
}
type ToolInfo struct{ Name, Description string }

var tools = []ToolInfo{
	{Name: "mysql_health", Description: "Check MySQL connectivity and server health."},
	{Name: "mysql_status", Description: "Read MySQL server status counters."},
	{Name: "mysql_performance", Description: "Read MySQL performance counters and configuration parameters."},
	{Name: "mysql_tables", Description: "List user tables with row counts, size and metadata."},
	{Name: "mysql_table_columns", Description: "Read columns, types, defaults and comments for one user table."},
	{Name: "mysql_table_structure", Description: "Read the CREATE TABLE definition and indexes for one user table."},
	{Name: "mysql_database_info", Description: "Read MySQL database identity and general information."},
}

func ListTools() []ToolInfo { return append([]ToolInfo(nil), tools...) }
func InputSchema(extra map[string]any) map[string]any {
	p := map[string]any{"host": map[string]any{"type": "string"}, "port": map[string]any{"type": "integer", "minimum": 1, "maximum": 65535}, "database": map[string]any{"type": "string"}, "username": map[string]any{"type": "string"}, "password": map[string]any{"type": "string"}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": 300}}
	for k, v := range extra {
		p[k] = v
	}
	return map[string]any{"type": "object", "properties": p, "additionalProperties": false}
}

func open(ctx context.Context, in ConnectionInput) (*sql.DB, error) {
	host, database, username := strings.TrimSpace(in.Host), strings.TrimSpace(in.Database), strings.TrimSpace(in.Username)
	if host == "" || database == "" || username == "" || in.Password == "" {
		return nil, errors.New("host, database, username and password are required")
	}
	port := in.Port
	if port <= 0 {
		port = 3306
	}
	if port > 65535 {
		return nil, errors.New("port must be between 1 and 65535")
	}
	cfg := driver.Config{User: username, Passwd: in.Password, Net: "tcp", Addr: net.JoinHostPort(host, strconv.Itoa(port)), DBName: database, Params: map[string]string{"parseTime": "true"}}
	if in.TimeoutSeconds > 0 {
		cfg.Timeout = time.Duration(in.TimeoutSeconds) * time.Second
		cfg.ReadTimeout = cfg.Timeout
		cfg.WriteTimeout = cfg.Timeout
	}
	dsn := cfg.FormatDSN()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if in.TimeoutSeconds > 0 {
		ctx2, cancel := context.WithTimeout(ctx, time.Duration(in.TimeoutSeconds)*time.Second)
		defer cancel()
		if err := db.PingContext(ctx2); err != nil {
			db.Close()
			return nil, err
		}
	} else if err := db.PingContext(ctx); err != nil {
		db.Close()
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
		switch x := v.(type) {
		case []byte:
			out[k] = string(x)
		default:
			out[k] = x
		}
	}
	return out, rows.Err()
}
func Health(ctx context.Context, in ConnectionInput) (HealthOutput, error) {
	started := time.Now()
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (HealthOutput, error) {
		var o HealthOutput
		if err := db.QueryRowContext(ctx, "SELECT VERSION(), DATABASE()").Scan(&o.Version, &o.Database); err != nil {
			return o, err
		}
		_ = db.QueryRowContext(ctx, "SELECT VARIABLE_VALUE FROM performance_schema.global_status WHERE VARIABLE_NAME='Uptime'").Scan(&o.UptimeSeconds)
		_ = db.QueryRowContext(ctx, "SELECT VARIABLE_VALUE FROM performance_schema.global_status WHERE VARIABLE_NAME='Threads_connected'").Scan(&o.ThreadsConnected)
		o.LatencyMS = time.Since(started).Milliseconds()
		return o, nil
	})
}
func Status(ctx context.Context, in ConnectionInput) (StatusOutput, error) {
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (StatusOutput, error) {
		v, e := queryMap(ctx, db, "SHOW GLOBAL STATUS WHERE Variable_name IN ('Threads_connected','Threads_running','Threads_created','Connections','Queries','Questions','Slow_queries','Aborted_connects','Innodb_buffer_pool_pages_dirty')")
		return StatusOutput{Values: v}, e
	})
}
func Performance(ctx context.Context, in ConnectionInput) (PerformanceOutput, error) {
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (PerformanceOutput, error) {
		v, e := queryMap(ctx, db, "SHOW GLOBAL VARIABLES WHERE Variable_name IN ('max_connections','innodb_buffer_pool_size','innodb_log_file_size','innodb_flush_log_at_trx_commit','slow_query_log','long_query_time','table_open_cache','query_cache_type','tmp_table_size','max_heap_table_size')")
		if e != nil {
			return PerformanceOutput{}, e
		}
		status, e := queryMap(ctx, db, "SHOW GLOBAL STATUS WHERE Variable_name IN ('Threads_connected','Threads_running','Questions','Com_select','Com_insert','Com_update','Com_delete','Created_tmp_tables','Created_tmp_disk_tables','Innodb_buffer_pool_read_requests','Innodb_buffer_pool_reads')")
		for k, val := range status {
			v["status."+k] = val
		}
		return PerformanceOutput{Values: v}, e
	})
}
func Tables(ctx context.Context, in ConnectionInput) (TablesOutput, error) {
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (TablesOutput, error) {
		rows, e := db.QueryContext(ctx, `SELECT TABLE_SCHEMA,TABLE_NAME,COALESCE(TABLE_ROWS,0),COALESCE(DATA_LENGTH,0)+COALESCE(INDEX_LENGTH,0),COALESCE(ENGINE,''),COALESCE(TABLE_COLLATION,''),COALESCE(DATE_FORMAT(CREATE_TIME,'%Y-%m-%dT%H:%i:%s'),''),COALESCE(DATE_FORMAT(UPDATE_TIME,'%Y-%m-%dT%H:%i:%s'),'') FROM information_schema.TABLES WHERE TABLE_SCHEMA NOT IN ('information_schema','mysql','performance_schema','sys') AND TABLE_TYPE='BASE TABLE' ORDER BY DATA_LENGTH+INDEX_LENGTH DESC LIMIT 1000`)
		if e != nil {
			return TablesOutput{}, e
		}
		defer rows.Close()
		out := TablesOutput{Tables: []TableInfo{}}
		for rows.Next() {
			var t TableInfo
			if e := rows.Scan(&t.Schema, &t.Name, &t.Rows, &t.DiskBytes, &t.Engine, &t.Collation, &t.CreatedAt, &t.UpdatedAt); e != nil {
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
		rows, e := db.QueryContext(ctx, `SELECT ORDINAL_POSITION,COLUMN_NAME,COLUMN_TYPE,IS_NULLABLE,COALESCE(COLUMN_DEFAULT,''),COALESCE(COLUMN_COMMENT,'') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? ORDER BY ORDINAL_POSITION`, input.Schema, input.Table)
		if e != nil {
			return TableColumnsOutput{}, e
		}
		defer rows.Close()
		out := TableColumnsOutput{Schema: input.Schema, Table: input.Table, Columns: []ColumnInfo{}}
		for rows.Next() {
			var c ColumnInfo
			if e := rows.Scan(&c.Position, &c.Name, &c.Type, &c.Nullable, &c.Default, &c.Comment); e != nil {
				return out, e
			}
			out.Columns = append(out.Columns, c)
		}
		return out, rows.Err()
	})
}
func TableStructure(ctx context.Context, in ConnectionInput, input TableColumnsInput) (TableStructureOutput, error) {
	input.Schema, input.Table = strings.TrimSpace(input.Schema), strings.TrimSpace(input.Table)
	if input.Schema == "" || input.Table == "" {
		return TableStructureOutput{}, errors.New("schema and table are required")
	}
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (TableStructureOutput, error) {
		var name, createSQL string
		if err := db.QueryRowContext(ctx, "SHOW CREATE TABLE `"+strings.ReplaceAll(input.Schema, "`", "``")+"`.`"+strings.ReplaceAll(input.Table, "`", "``")+"`").Scan(&name, &createSQL); err != nil {
			return TableStructureOutput{}, err
		}
		rows, err := db.QueryContext(ctx, "SHOW INDEX FROM `"+strings.ReplaceAll(input.Table, "`", "``")+"` FROM `"+strings.ReplaceAll(input.Schema, "`", "``")+"`")
		if err != nil {
			return TableStructureOutput{}, err
		}
		defer rows.Close()
		out := TableStructureOutput{Schema: input.Schema, Table: input.Table, CreateSQL: createSQL, Indexes: []map[string]any{}}
		cols, err := rows.Columns()
		if err != nil {
			return out, err
		}
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				return out, err
			}
			item := map[string]any{}
			for i, col := range cols {
				if b, ok := vals[i].([]byte); ok {
					item[col] = string(b)
				} else {
					item[col] = vals[i]
				}
			}
			out.Indexes = append(out.Indexes, item)
		}
		return out, rows.Err()
	})
}
func DatabaseInfo(ctx context.Context, in ConnectionInput) (DatabaseInfoOutput, error) {
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (DatabaseInfoOutput, error) {
		v, e := queryMap(ctx, db, "SHOW GLOBAL VARIABLES WHERE Variable_name IN ('version','version_comment','character_set_server','collation_server','default_storage_engine','sql_mode','time_zone','lower_case_table_names')")
		return DatabaseInfoOutput{Values: v}, e
	})
}
