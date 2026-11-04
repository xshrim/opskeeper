package postgresql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TableStructure(ctx context.Context, connection ConnectionInput, input TableStructureInput) (TableStructureOutput, error) {
	input.Schema, input.Table = strings.TrimSpace(input.Schema), strings.TrimSpace(input.Table)
	if input.Schema == "" || input.Table == "" {
		return TableStructureOutput{}, errors.New("schema and table are required")
	}
	return withConn(ctx, connection, func(ctx context.Context, conn *pgx.Conn) (TableStructureOutput, error) {
		out := TableStructureOutput{Schema: input.Schema, Table: input.Table, Constraints: []ConstraintInfo{}, Indexes: []IndexInfo{}}
		var oid uint32
		if err := conn.QueryRow(ctx, `SELECT c.oid FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2 AND c.relkind IN ('r','p') AND has_table_privilege(c.oid,'SELECT')`, input.Schema, input.Table).Scan(&oid); err != nil {
			return out, fmt.Errorf("find visible user table: %w", err)
		}
		rows, err := conn.Query(ctx, `SELECT conname,contype::text,''::text,pg_get_constraintdef(oid),''::text,''::text FROM pg_constraint WHERE conrelid=$1 ORDER BY conname LIMIT 1000`, oid)
		if err != nil {
			return out, err
		}
		out.Constraints, err = pgx.CollectRows(rows, pgx.RowToStructByPos[ConstraintInfo])
		if err != nil {
			return out, err
		}
		rows, err = conn.Query(ctx, `SELECT ci.relname,am.amname,''::text,pg_get_indexdef(i.indexrelid),pg_relation_size(i.indexrelid),i.indisprimary,i.indisunique,CASE WHEN i.indisready THEN 'ready' ELSE 'not_ready' END FROM pg_index i JOIN pg_class ci ON ci.oid=i.indexrelid JOIN pg_am am ON am.oid=ci.relam WHERE i.indrelid=$1 ORDER BY ci.relname LIMIT 1000`, oid)
		if err != nil {
			return out, err
		}
		out.Indexes, err = pgx.CollectRows(rows, pgx.RowToStructByPos[IndexInfo])
		return out, err
	})
}

func TableSchemaLookup(ctx context.Context, connection ConnectionInput, input SchemaLookupInput) (SchemaLookupOutput, error) {
	input.Table = strings.TrimSpace(input.Table)
	if input.Table == "" {
		return SchemaLookupOutput{}, errors.New("table is required")
	}
	return withConn(ctx, connection, func(ctx context.Context, conn *pgx.Conn) (SchemaLookupOutput, error) {
		out := SchemaLookupOutput{Table: input.Table, Matches: []SchemaMatch{}}
		rows, err := conn.Query(ctx, `SELECT n.nspname,c.relname,c.relkind::text FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relname=$1 AND c.relkind IN ('r','p','v','m','f') AND n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_toast' AND has_schema_privilege(n.oid,'USAGE') AND has_table_privilege(c.oid,'SELECT') ORDER BY n.nspname LIMIT 1000`, input.Table)
		if err != nil {
			return out, err
		}
		out.Matches, err = pgx.CollectRows(rows, pgx.RowToStructByPos[SchemaMatch])
		return out, err
	})
}

func SlowQueries(ctx context.Context, connection ConnectionInput, input SlowQueriesInput) (SlowQueriesOutput, error) {
	if input.Limit == 0 {
		input.Limit = 20
	}
	if input.Limit < 1 || input.Limit > 100 || input.MinTotalMS < 0 || math.IsNaN(input.MinTotalMS) || math.IsInf(input.MinTotalMS, 0) {
		return SlowQueriesOutput{}, errors.New("limit must be 1..100 and min_total_ms must be finite and non-negative")
	}
	return withConn(ctx, connection, func(ctx context.Context, conn *pgx.Conn) (SlowQueriesOutput, error) {
		out := SlowQueriesOutput{Queries: []SlowQueryInfo{}}
		var schema string
		err := conn.QueryRow(ctx, `SELECT n.nspname FROM pg_extension e JOIN pg_namespace n ON n.oid=e.extnamespace WHERE e.extname='pg_stat_statements'`).Scan(&schema)
		if errors.Is(err, pgx.ErrNoRows) {
			out.Reason = "pg_stat_statements is not installed in this database"
			return out, nil
		}
		if err != nil {
			return out, err
		}
		// PostgreSQL 13+ uses *_exec_time. Older extension versions use *_time.
		var modern, hasQueryID bool
		err = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=$1 AND table_name='pg_stat_statements' AND column_name='total_exec_time'),EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=$1 AND table_name='pg_stat_statements' AND column_name='queryid')`, schema).Scan(&modern, &hasQueryID)
		if err != nil {
			return out, err
		}
		total, mean, max := "total_exec_time", "mean_exec_time", "max_exec_time"
		if !modern {
			total, mean, max = "total_time", "mean_time", "max_time"
		}
		queryID := `''::text`
		if hasQueryID {
			queryID = `COALESCE(queryid::text,'')`
		}
		q := fmt.Sprintf(`SELECT %s,calls,%s::float8,%s::float8,%s::float8,rows,LEFT(query,4096) FROM %s WHERE dbid=(SELECT oid FROM pg_database WHERE datname=current_database()) AND %s >= $1 ORDER BY %s DESC LIMIT $2`, queryID, total, mean, max, pgx.Identifier{schema, "pg_stat_statements"}.Sanitize(), total, total)
		rows, err := conn.Query(ctx, q, input.MinTotalMS, input.Limit)
		if err == nil {
			out.Queries, err = pgx.CollectRows(rows, pgx.RowToStructByPos[SlowQueryInfo])
		}
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && (pgErr.Code == "42501" || pgErr.Code == "55000" || pgErr.Code == "42P01" || pgErr.Code == "42703") {
				out.Reason = pgErr.Message
				return out, nil
			}
			return out, err
		}
		out.Available = true
		return out, nil
	})
}

func ActiveQueries(ctx context.Context, connection ConnectionInput) (ActiveQueriesOutput, error) {
	return withConn(ctx, connection, func(ctx context.Context, conn *pgx.Conn) (ActiveQueriesOutput, error) {
		out := ActiveQueriesOutput{Available: true, Queries: []ActiveQueryInfo{}}
		rows, err := conn.Query(ctx, `SELECT pid::bigint,COALESCE(usename,''),COALESCE(application_name,''),COALESCE(client_addr::text,''),COALESCE(state,''),COALESCE(wait_event_type,''),COALESCE(wait_event,''),COALESCE(query_start::text,''),COALESCE(EXTRACT(EPOCH FROM (clock_timestamp()-query_start))::bigint,0),LEFT(COALESCE(query,''),4096) FROM pg_stat_activity WHERE datname=current_database() AND state='active' AND pid<>pg_backend_pid() ORDER BY query_start LIMIT 100`)
		if err != nil {
			out.Available, out.Reason = false, err.Error()
			return out, nil
		}
		out.Queries, err = pgx.CollectRows(rows, pgx.RowToStructByPos[ActiveQueryInfo])
		return out, err
	})
}

func LockWaits(ctx context.Context, connection ConnectionInput) (LockWaitsOutput, error) {
	return withConn(ctx, connection, func(ctx context.Context, conn *pgx.Conn) (LockWaitsOutput, error) {
		out := LockWaitsOutput{Available: true, Waits: []LockWaitInfo{}}
		rows, err := conn.Query(ctx, `SELECT a.pid::bigint,b.pid::bigint,COALESCE(a.usename,''),COALESCE(b.usename,''),LEFT(COALESCE(a.query,''),4096),LEFT(COALESCE(b.query,''),4096),0::bigint,false,''::text,''::text FROM pg_stat_activity a CROSS JOIN LATERAL unnest(pg_blocking_pids(a.pid)) p(pid) JOIN pg_stat_activity b ON b.pid=p.pid WHERE a.datname=current_database() AND a.pid<>pg_backend_pid() ORDER BY a.query_start LIMIT 100`)
		if err != nil {
			return out, err
		}
		out.Waits, err = pgx.CollectRows(rows, pgx.RowToStructByPos[LockWaitInfo])
		return out, err
	})
}

func VacuumTableStats(ctx context.Context, connection ConnectionInput) (VacuumTableStatsOutput, error) {
	return withConn(ctx, connection, func(ctx context.Context, conn *pgx.Conn) (VacuumTableStatsOutput, error) {
		out := VacuumTableStatsOutput{Tables: []VacuumTableStat{}}
		rows, err := conn.Query(ctx, `SELECT schemaname,relname,n_live_tup,n_dead_tup,n_mod_since_analyze,COALESCE(last_vacuum::text,''),COALESCE(last_autovacuum::text,''),COALESCE(last_analyze::text,''),COALESCE(last_autoanalyze::text,''),vacuum_count,autovacuum_count FROM pg_stat_user_tables ORDER BY n_dead_tup DESC LIMIT 1000`)
		if err != nil {
			return out, err
		}
		out.Tables, err = pgx.CollectRows(rows, pgx.RowToStructByPos[VacuumTableStat])
		return out, err
	})
}

func ConnectionStats(ctx context.Context, connection ConnectionInput) (ConnectionStatsOutput, error) {
	return withConn(ctx, connection, func(ctx context.Context, conn *pgx.Conn) (ConnectionStatsOutput, error) {
		out := ConnectionStatsOutput{Available: true, CapacityKnown: true}
		err := conn.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE state='active'),count(*) FILTER(WHERE state='idle'),count(*) FILTER(WHERE state IN ('idle in transaction','idle in transaction (aborted)')),current_setting('max_connections')::bigint FROM pg_stat_activity WHERE backend_type='client backend' AND pid<>pg_backend_pid()`).Scan(&out.Total, &out.Active, &out.Idle, &out.IdleInTransaction, &out.Max)
		if out.Max > 0 {
			out.UtilizationPct = float64(out.Total) * 100 / float64(out.Max)
		}
		return out, err
	})
}

func ConnectionGroups(ctx context.Context, connection ConnectionInput) (ConnectionGroupsOutput, error) {
	return withConn(ctx, connection, func(ctx context.Context, conn *pgx.Conn) (ConnectionGroupsOutput, error) {
		out := ConnectionGroupsOutput{Groups: []ConnectionGroup{}}
		rows, err := conn.Query(ctx, `SELECT count(*),COALESCE(state,''),COALESCE(usename,''),COALESCE(application_name,''),COALESCE(client_addr::text,''),COALESCE(wait_event_type,''),COALESCE(wait_event,'') FROM pg_stat_activity WHERE backend_type='client backend' AND pid<>pg_backend_pid() GROUP BY state,usename,application_name,client_addr,wait_event_type,wait_event ORDER BY count(*) DESC LIMIT 100`)
		if err != nil {
			return out, err
		}
		out.Groups, err = pgx.CollectRows(rows, pgx.RowToStructByPos[ConnectionGroup])
		return out, err
	})
}

func IdleSessions(ctx context.Context, connection ConnectionInput) (IdleSessionsOutput, error) {
	return withConn(ctx, connection, func(ctx context.Context, conn *pgx.Conn) (IdleSessionsOutput, error) {
		out := IdleSessionsOutput{Sessions: []IdleSession{}}
		rows, err := conn.Query(ctx, `SELECT pid::bigint,COALESCE(usename,''),COALESCE(application_name,''),COALESCE(client_addr::text,''),state,COALESCE(state_change::text,''),COALESCE(query_start::text,''),LEFT(COALESCE(query,''),4096) FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND state IN ('idle','idle in transaction','idle in transaction (aborted)') ORDER BY state_change LIMIT 100`)
		if err != nil {
			return out, err
		}
		out.Sessions, err = pgx.CollectRows(rows, pgx.RowToStructByPos[IdleSession])
		return out, err
	})
}

func Explain(ctx context.Context, connection ConnectionInput, input ExplainInput) (ExplainOutput, error) {
	query, err := validateExplainQuery(input.Query)
	if err != nil {
		return ExplainOutput{}, err
	}
	return withConn(ctx, connection, func(ctx context.Context, conn *pgx.Conn) (ExplainOutput, error) {
		out := ExplainOutput{Schema: strings.TrimSpace(input.Schema)}
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
		if err != nil {
			return out, err
		}
		defer tx.Rollback(context.WithoutCancel(ctx))
		if _, err = tx.Exec(ctx, `SET LOCAL statement_timeout='5s'`); err != nil {
			return out, err
		}
		if _, err = tx.Exec(ctx, `SET LOCAL lock_timeout='1s'`); err != nil {
			return out, err
		}
		if out.Schema != "" {
			if _, err = tx.Exec(ctx, `SELECT set_config('search_path',$1,true)`, pgx.Identifier{out.Schema}.Sanitize()); err != nil {
				return out, err
			}
		}
		// Extended protocol rejects multiple SQL statements even if validation changes.
		var plan json.RawMessage
		err = tx.QueryRow(ctx, "EXPLAIN (FORMAT JSON, VERBOSE) "+query).Scan(&plan)
		if err == nil {
			err = json.Unmarshal(plan, &out.Plan)
		}
		return out, err
	})
}

func validateExplainQuery(raw string) (string, error) {
	query := strings.TrimSpace(raw)
	if query == "" {
		return "", errors.New("query is required")
	}
	if len(query) > 10000 {
		return "", errors.New("query must not exceed 10000 characters")
	}
	if strings.Contains(query, ";") || strings.Contains(query, "--") || strings.Contains(query, "/*") || strings.Contains(query, "*/") {
		return "", errors.New("query must be one SELECT statement without comments or semicolons")
	}
	upper := strings.ToUpper(query)
	fields := strings.Fields(upper)
	if len(fields) == 0 || (fields[0] != "SELECT" && fields[0] != "WITH") {
		return "", errors.New("only SELECT or read-only WITH queries are allowed")
	}
	for _, word := range []string{"INSERT", "UPDATE", "DELETE", "MERGE", "UPSERT", "DROP", "ALTER", "CREATE", "TRUNCATE", "VACUUM", "GRANT", "REVOKE", "COPY", "CALL", "DO", "EXECUTE", "REFRESH"} {
		if containsSQLWord(upper, word) {
			return "", fmt.Errorf("query contains prohibited keyword %s", word)
		}
	}
	return query, nil
}

func containsSQLWord(query, word string) bool {
	for i := 0; i+len(word) <= len(query); i++ {
		if query[i:i+len(word)] != word {
			continue
		}
		beforeOK := i == 0 || (query[i-1] < 'A' || query[i-1] > 'Z') && (query[i-1] < '0' || query[i-1] > '9') && query[i-1] != '_'
		after := i + len(word)
		afterOK := after == len(query) || (query[after] < 'A' || query[after] > 'Z') && (query[after] < '0' || query[after] > '9') && query[after] != '_'
		if beforeOK && afterOK {
			return true
		}
	}
	return false
}
