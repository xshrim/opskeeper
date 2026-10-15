// Package oceanbase provides the fixed, read-only OceanBase inspection tools.
package oceanbase

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/helingjun/obconnector-go"
	_ "github.com/sijms/go-ora/v2"
)

const (
	ModeMySQL  = "mysql"
	ModeOracle = "oracle"
)

type ConnectionInput struct {
	Host           string `json:"host,omitempty"`
	Port           int    `json:"port,omitempty"`
	Mode           string `json:"mode,omitempty"`
	Database       string `json:"database,omitempty"`
	ServiceName    string `json:"service_name,omitempty"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

type HealthOutput struct {
	Mode      string `json:"mode"`
	Route     string `json:"route"`
	Version   string `json:"version"`
	Database  string `json:"database"`
	LatencyMS int64  `json:"latency_ms"`
}
type ValuesOutput struct {
	Values map[string]any `json:"values"`
}
type TableInfo struct {
	Schema    string `json:"schema"`
	Name      string `json:"name"`
	Rows      int64  `json:"rows"`
	DiskBytes int64  `json:"disk_bytes"`
	Engine    string `json:"engine,omitempty"`
	Status    string `json:"status,omitempty"`
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
	Schema      string           `json:"schema"`
	Table       string           `json:"table"`
	CreateSQL   string           `json:"create_sql,omitempty"`
	Constraints []map[string]any `json:"constraints,omitempty"`
	Indexes     []map[string]any `json:"indexes"`
}
type ToolInfo struct{ Name, Description string }

var tools = []ToolInfo{
	{Name: "oceanbase_health", Description: "Check OceanBase connectivity and server health."},
	{Name: "oceanbase_status", Description: "Read OceanBase server status counters."},
	{Name: "oceanbase_performance", Description: "Read OceanBase performance counters and configuration."},
	{Name: "oceanbase_database_info", Description: "Read OceanBase database identity and configuration."},
	{Name: "oceanbase_tables", Description: "List OceanBase user tables with row counts and size."},
	{Name: "oceanbase_table_columns", Description: "Read columns, types, defaults and comments for one table."},
	{Name: "oceanbase_table_structure", Description: "Read indexes and constraints for one table."},
}

func ListTools() []ToolInfo { return append([]ToolInfo(nil), tools...) }
func InputSchema(extra map[string]any) map[string]any {
	p := map[string]any{"host": map[string]any{"type": "string"}, "port": map[string]any{"type": "integer", "minimum": 1, "maximum": 65535}, "mode": map[string]any{"type": "string", "enum": []string{ModeMySQL, ModeOracle}}, "database": map[string]any{"type": "string"}, "service_name": map[string]any{"type": "string"}, "username": map[string]any{"type": "string"}, "password": map[string]any{"type": "string"}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": 300}}
	for k, v := range extra {
		p[k] = v
	}
	return map[string]any{"type": "object", "properties": p, "additionalProperties": false}
}

func mode(in ConnectionInput) (string, error) {
	m := strings.ToLower(strings.TrimSpace(in.Mode))
	if m == "" {
		m = ModeMySQL
	}
	if m != ModeMySQL && m != ModeOracle {
		return "", errors.New("mode must be mysql or oracle")
	}
	return m, nil
}
func timeout(in ConnectionInput) time.Duration {
	if in.TimeoutSeconds > 0 {
		return time.Duration(in.TimeoutSeconds) * time.Second
	}
	return 10 * time.Second
}
func address(in ConnectionInput) (string, error) {
	if strings.TrimSpace(in.Host) == "" || strings.TrimSpace(in.Username) == "" || in.Password == "" {
		return "", errors.New("host, username and password are required")
	}
	p := in.Port
	if p == 0 {
		p = 2881
	}
	if p < 1 || p > 65535 {
		return "", errors.New("port must be between 1 and 65535")
	}
	return net.JoinHostPort(strings.TrimSpace(in.Host), strconv.Itoa(p)), nil
}

// open selects the wire protocol once. Oracle-mode resources use OBClient on
// MySQL-wire ports and TNS only on non-MySQL-wire ports.
func open(ctx context.Context, in ConnectionInput) (*sql.DB, string, string, error) {
	m, err := mode(in)
	if err != nil {
		return nil, "", "", err
	}
	addr, err := address(in)
	if err != nil {
		return nil, "", "", err
	}
	if m == ModeMySQL {
		if strings.TrimSpace(in.Database) == "" {
			return nil, "", "", errors.New("database is required for mysql mode")
		}
		dsn := url.QueryEscape(in.Username) + ":" + url.QueryEscape(in.Password) + "@tcp(" + addr + ")/" + url.QueryEscape(in.Database) + "?parseTime=true&timeout=" + timeout(in).String() + "&readTimeout=" + timeout(in).String() + "&writeTimeout=" + timeout(in).String()
		return openAndPing(ctx, "mysql", dsn, timeout(in), m, "mysql-wire")
	}
	mysqlWire, reachable, probeErr := probeMySQLWire(ctx, addr, timeout(in))
	if !reachable {
		return nil, "", "", fmt.Errorf("OceanBase endpoint is unreachable: %w", probeErr)
	}
	if mysqlWire {
		u := url.URL{Scheme: "oboracle", Host: addr, User: url.UserPassword(in.Username, in.Password)}
		if strings.TrimSpace(in.Database) != "" {
			u.Path = "/" + strings.TrimSpace(in.Database)
		}
		q := u.Query()
		q.Set("preset", "oboracle")
		q.Set("timeout", timeout(in).String())
		u.RawQuery = q.Encode()
		return openAndPing(ctx, "oboracle", u.String(), timeout(in), m, "obclient-mysql-wire")
	}
	if strings.TrimSpace(in.ServiceName) == "" {
		return nil, "", "", errors.New("service_name is required when the Oracle endpoint is not a MySQL Wire port")
	}
	dsn := fmt.Sprintf("oracle://%s:%s@%s/%s", url.PathEscape(in.Username), url.PathEscape(in.Password), addr, url.PathEscape(in.ServiceName))
	return openAndPing(ctx, "oracle", dsn, timeout(in), m, "oracle-tns")
}
func openAndPing(ctx context.Context, driver, dsn string, wait time.Duration, m, route string) (*sql.DB, string, string, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, "", "", err
	}
	c, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	if err = db.PingContext(c); err != nil {
		_ = db.Close()
		return nil, "", "", err
	}
	return db, m, route, nil
}
func probeMySQLWire(ctx context.Context, addr string, wait time.Duration) (bool, bool, error) {
	if wait > 3*time.Second {
		wait = 3 * time.Second
	}
	c, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(c, "tcp", addr)
	if err != nil {
		return false, false, err
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(wait))
	header := make([]byte, 4)
	if _, err = io.ReadFull(conn, header); err != nil {
		return false, true, err
	}
	n := int(header[0]) | int(header[1])<<8 | int(header[2])<<16
	if n < 2 || n > 65536 {
		return false, true, nil
	}
	body := make([]byte, n)
	if _, err = io.ReadFull(conn, body); err != nil {
		return false, true, err
	}
	return body[0] != 0 && bytes.IndexByte(body[1:], 0) >= 0, true, nil
}
func withDB[T any](ctx context.Context, in ConnectionInput, fn func(context.Context, *sql.DB, string, string) (T, error)) (T, error) {
	var zero T
	db, m, route, err := open(ctx, in)
	if err != nil {
		return zero, err
	}
	defer db.Close()
	return fn(ctx, db, m, route)
}
func mapRows(ctx context.Context, db *sql.DB, q string, args ...any) (map[string]any, error) {
	rows, err := db.QueryContext(ctx, q, args...)
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
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB, m, route string) (HealthOutput, error) {
		out := HealthOutput{Mode: m, Route: route, LatencyMS: time.Since(started).Milliseconds()}
		var err error
		if m == ModeMySQL {
			err = db.QueryRowContext(ctx, "SELECT VERSION(), DATABASE()").Scan(&out.Version, &out.Database)
		} else {
			err = db.QueryRowContext(ctx, "SELECT banner FROM v$version WHERE ROWNUM=1").Scan(&out.Version)
			_ = db.QueryRowContext(ctx, "SELECT SYS_CONTEXT('USERENV','DB_NAME') FROM dual").Scan(&out.Database)
		}
		return out, err
	})
}
func Status(ctx context.Context, in ConnectionInput) (ValuesOutput, error) {
	return values(ctx, in, "SHOW GLOBAL STATUS WHERE Variable_name IN ('Threads_connected','Threads_running','Connections','Queries','Slow_queries')", `SELECT 'instance_status',status FROM v$instance UNION ALL SELECT 'active_sessions',TO_CHAR(COUNT(*)) FROM v$session WHERE status='ACTIVE' UNION ALL SELECT 'transactions',TO_CHAR(COUNT(*)) FROM v$transaction`)
}
func Performance(ctx context.Context, in ConnectionInput) (ValuesOutput, error) {
	return values(ctx, in, "SHOW GLOBAL VARIABLES WHERE Variable_name IN ('max_connections','innodb_buffer_pool_size','slow_query_log','long_query_time')", `SELECT name,value FROM v$parameter WHERE name IN ('processes','sessions','open_cursors','optimizer_mode') UNION ALL SELECT name,TO_CHAR(value) FROM v$sysstat WHERE name IN ('physical reads','physical writes','session logical reads','execute count')`)
}
func DatabaseInfo(ctx context.Context, in ConnectionInput) (ValuesOutput, error) {
	return values(ctx, in, "SHOW GLOBAL VARIABLES WHERE Variable_name IN ('version','version_comment','character_set_server','collation_server','sql_mode','time_zone')", `SELECT 'db_name',name FROM v$database UNION ALL SELECT 'role',database_role FROM v$database UNION ALL SELECT 'open_mode',open_mode FROM v$database UNION ALL SELECT 'version',version FROM v$instance`)
}
func values(ctx context.Context, in ConnectionInput, mysql, oracle string) (ValuesOutput, error) {
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB, m, _ string) (ValuesOutput, error) {
		q := mysql
		if m == ModeOracle {
			q = oracle
		}
		v, e := mapRows(ctx, db, q)
		return ValuesOutput{Values: v}, e
	})
}
func Tables(ctx context.Context, in ConnectionInput) (TablesOutput, error) {
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB, m, _ string) (TablesOutput, error) {
		q := `SELECT TABLE_SCHEMA,TABLE_NAME,COALESCE(TABLE_ROWS,0),COALESCE(DATA_LENGTH,0)+COALESCE(INDEX_LENGTH,0),COALESCE(ENGINE,'') FROM information_schema.TABLES WHERE TABLE_SCHEMA NOT IN ('information_schema','mysql','performance_schema','sys') AND TABLE_TYPE='BASE TABLE' ORDER BY DATA_LENGTH+INDEX_LENGTH DESC LIMIT 1000`
		if m == ModeOracle {
			q = `SELECT owner,table_name,COALESCE(num_rows,0),0,COALESCE(status,'') FROM all_tables WHERE owner=SYS_CONTEXT('USERENV','CURRENT_SCHEMA') AND temporary='N' FETCH FIRST 1000 ROWS ONLY`
		}
		rows, e := db.QueryContext(ctx, q)
		if e != nil {
			return TablesOutput{}, e
		}
		defer rows.Close()
		out := TablesOutput{Tables: []TableInfo{}}
		for rows.Next() {
			var t TableInfo
			if e := rows.Scan(&t.Schema, &t.Name, &t.Rows, &t.DiskBytes, &t.Engine); e != nil {
				return out, e
			}
			out.Tables = append(out.Tables, t)
		}
		return out, rows.Err()
	})
}
func TableColumns(ctx context.Context, in ConnectionInput, input TableColumnsInput) (TableColumnsOutput, error) {
	input.Schema = strings.TrimSpace(input.Schema)
	input.Table = strings.TrimSpace(input.Table)
	if input.Schema == "" || input.Table == "" {
		return TableColumnsOutput{}, errors.New("schema and table are required")
	}
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB, m, _ string) (TableColumnsOutput, error) {
		q := `SELECT ORDINAL_POSITION,COLUMN_NAME,COLUMN_TYPE,IS_NULLABLE,COALESCE(COLUMN_DEFAULT,''),COALESCE(COLUMN_COMMENT,'') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? ORDER BY ORDINAL_POSITION`
		args := []any{input.Schema, input.Table}
		if m == ModeOracle {
			q = `SELECT column_id,column_name,data_type,nullable,COALESCE(data_default,''),'' FROM all_tab_columns WHERE owner=:1 AND table_name=:2 ORDER BY column_id`
			args = []any{strings.ToUpper(input.Schema), strings.ToUpper(input.Table)}
		}
		rows, e := db.QueryContext(ctx, q, args...)
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
	input.Schema = strings.TrimSpace(input.Schema)
	input.Table = strings.TrimSpace(input.Table)
	if input.Schema == "" || input.Table == "" {
		return TableStructureOutput{}, errors.New("schema and table are required")
	}
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB, m, _ string) (TableStructureOutput, error) {
		out := TableStructureOutput{Schema: input.Schema, Table: input.Table, Indexes: []map[string]any{}, Constraints: []map[string]any{}}
		if m == ModeMySQL {
			var n string
			e := db.QueryRowContext(ctx, "SHOW CREATE TABLE `"+strings.ReplaceAll(input.Schema, "`", "``")+"`.`"+strings.ReplaceAll(input.Table, "`", "``")+"`").Scan(&n, &out.CreateSQL)
			if e != nil {
				return out, e
			}
			rows, e := db.QueryContext(ctx, "SHOW INDEX FROM `"+strings.ReplaceAll(input.Table, "`", "``")+"` FROM `"+strings.ReplaceAll(input.Schema, "`", "``")+"`")
			if e != nil {
				return out, e
			}
			defer rows.Close()
			return out, scanMaps(rows, &out.Indexes)
		}
		for _, entry := range []struct {
			q   string
			dst *[]map[string]any
		}{{`SELECT constraint_name,constraint_type,status FROM all_constraints WHERE owner=:1 AND table_name=:2 ORDER BY constraint_name`, &out.Constraints}, {`SELECT index_name,index_type,status FROM all_indexes WHERE owner=:1 AND table_name=:2 ORDER BY index_name`, &out.Indexes}} {
			rows, e := db.QueryContext(ctx, entry.q, strings.ToUpper(input.Schema), strings.ToUpper(input.Table))
			if e != nil {
				return out, e
			}
			if e = scanMaps(rows, entry.dst); e != nil {
				rows.Close()
				return out, e
			}
			rows.Close()
		}
		return out, nil
	})
}
func scanMaps(rows *sql.Rows, dst *[]map[string]any) error {
	cols, e := rows.Columns()
	if e != nil {
		return e
	}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if e = rows.Scan(ptrs...); e != nil {
			return e
		}
		item := map[string]any{}
		for i, col := range cols {
			if b, ok := vals[i].([]byte); ok {
				item[col] = string(b)
			} else {
				item[col] = vals[i]
			}
		}
		*dst = append(*dst, item)
	}
	return rows.Err()
}
