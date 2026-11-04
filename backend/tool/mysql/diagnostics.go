package mysql

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strings"

	"opskeeper/backend/tool/relational"
)

type SlowQueriesInput = relational.SlowQueriesInput
type SlowQueriesOutput = relational.SlowQueriesOutput
type ActiveQueriesOutput = relational.ActiveQueriesOutput
type LockWaitsOutput = relational.LockWaitsOutput
type ConnectionStatsOutput = relational.ConnectionStatsOutput

func SlowQueries(ctx context.Context, in ConnectionInput, input SlowQueriesInput) (SlowQueriesOutput, error) {
	if input.Limit == 0 {
		input.Limit = 20
	}
	if input.Limit < 1 || input.Limit > 100 || input.MinTotalMS < 0 || math.IsNaN(input.MinTotalMS) || math.IsInf(input.MinTotalMS, 0) {
		return SlowQueriesOutput{}, errors.New("limit must be 1..100 and min_total_ms must be finite and non-negative")
	}
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (SlowQueriesOutput, error) {
		out := SlowQueriesOutput{Available: true, Queries: []relational.SlowQuery{}}
		rows, err := db.QueryContext(ctx, `SELECT COALESCE(DIGEST,''),COUNT_STAR,SUM_TIMER_WAIT/1000000000.0,AVG_TIMER_WAIT/1000000000.0,MAX_TIMER_WAIT/1000000000.0,SUM_ROWS_SENT,LEFT(COALESCE(DIGEST_TEXT,''),4096) FROM performance_schema.events_statements_summary_by_digest WHERE SCHEMA_NAME=DATABASE() AND SUM_TIMER_WAIT/1000000000.0>=? ORDER BY SUM_TIMER_WAIT DESC LIMIT ?`, input.MinTotalMS, input.Limit)
		if err != nil {
			out.Available, out.Reason = false, err.Error()
			return out, nil
		}
		defer rows.Close()
		for rows.Next() {
			var item relational.SlowQuery
			if err := rows.Scan(&item.QueryID, &item.Calls, &item.TotalExecMS, &item.MeanExecMS, &item.MaxExecMS, &item.Rows, &item.Query); err != nil {
				return out, err
			}
			out.Queries = append(out.Queries, item)
		}
		return out, rows.Err()
	})
}

func ActiveQueries(ctx context.Context, in ConnectionInput) (ActiveQueriesOutput, error) {
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (ActiveQueriesOutput, error) {
		out := ActiveQueriesOutput{Available: true, Queries: []relational.ActiveQuery{}}
		rows, err := db.QueryContext(ctx, `SELECT ID,COALESCE(USER,''),'',COALESCE(HOST,''),COALESCE(STATE,''),'','',COALESCE(DATE_FORMAT(DATE_SUB(NOW(),INTERVAL TIME SECOND),'%Y-%m-%d %H:%i:%s'),''),COALESCE(TIME,0),LEFT(COALESCE(INFO,''),4096) FROM information_schema.PROCESSLIST WHERE ID<>CONNECTION_ID() AND COMMAND<>'Sleep' ORDER BY TIME DESC LIMIT 100`)
		if err != nil {
			out.Available, out.Reason = false, err.Error()
			return out, nil
		}
		defer rows.Close()
		for rows.Next() {
			var item relational.ActiveQuery
			if err := rows.Scan(&item.SessionID, &item.User, &item.ApplicationName, &item.ClientAddress, &item.State, &item.WaitEventType, &item.WaitEvent, &item.QueryStart, &item.DurationSeconds, &item.Query); err != nil {
				return out, err
			}
			out.Queries = append(out.Queries, item)
		}
		return out, rows.Err()
	})
}

func LockWaits(ctx context.Context, in ConnectionInput) (LockWaitsOutput, error) {
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (LockWaitsOutput, error) {
		out := LockWaitsOutput{Available: true, Waits: []relational.LockWait{}}
		rows, err := db.QueryContext(ctx, `SELECT wt.PROCESSLIST_ID,bt.PROCESSLIST_ID,COALESCE(wt.PROCESSLIST_USER,''),COALESCE(bt.PROCESSLIST_USER,''),LEFT(COALESCE(ws.SQL_TEXT,''),4096),LEFT(COALESCE(bs.SQL_TEXT,''),4096),COALESCE(TIMESTAMPDIFF(SECOND,trxw.TRX_WAIT_STARTED,NOW()),0),trxw.TRX_WAIT_STARTED IS NOT NULL,'','' FROM performance_schema.data_lock_waits w JOIN performance_schema.data_locks wl ON wl.ENGINE=w.ENGINE AND wl.ENGINE_LOCK_ID=w.REQUESTING_ENGINE_LOCK_ID JOIN performance_schema.data_locks bl ON bl.ENGINE=w.ENGINE AND bl.ENGINE_LOCK_ID=w.BLOCKING_ENGINE_LOCK_ID JOIN performance_schema.threads wt ON wt.THREAD_ID=wl.THREAD_ID JOIN performance_schema.threads bt ON bt.THREAD_ID=bl.THREAD_ID LEFT JOIN performance_schema.events_statements_current ws ON ws.THREAD_ID=wt.THREAD_ID LEFT JOIN performance_schema.events_statements_current bs ON bs.THREAD_ID=bt.THREAD_ID LEFT JOIN information_schema.innodb_trx trxw ON trxw.TRX_MYSQL_THREAD_ID=wt.PROCESSLIST_ID ORDER BY trxw.TRX_WAIT_STARTED LIMIT 100`)
		if err != nil {
			out.Available, out.Reason = false, err.Error()
			return out, nil
		}
		defer rows.Close()
		for rows.Next() {
			var item relational.LockWait
			if err := rows.Scan(&item.BlockedSessionID, &item.BlockingSessionID, &item.BlockedUser, &item.BlockingUser, &item.BlockedQuery, &item.BlockingQuery, &item.WaitDurationSeconds, &item.WaitDurationKnown, &item.LockType, &item.LockResource); err != nil {
				return out, err
			}
			out.Waits = append(out.Waits, item)
		}
		return out, rows.Err()
	})
}

func ConnectionStats(ctx context.Context, in ConnectionInput) (ConnectionStatsOutput, error) {
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (ConnectionStatsOutput, error) {
		out := ConnectionStatsOutput{Available: true, CapacityKnown: true}
		err := db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(COMMAND<>'Sleep'),0),COALESCE(SUM(COMMAND='Sleep' AND ID NOT IN (SELECT trx_mysql_thread_id FROM information_schema.innodb_trx)),0),COALESCE(SUM(COMMAND='Sleep' AND ID IN (SELECT trx_mysql_thread_id FROM information_schema.innodb_trx)),0) FROM information_schema.PROCESSLIST WHERE ID<>CONNECTION_ID()`).Scan(&out.Total, &out.Active, &out.Idle, &out.IdleInTransaction)
		if err != nil {
			return out, err
		}
		err = db.QueryRowContext(ctx, `SELECT @@GLOBAL.max_connections`).Scan(&out.Max)
		if err != nil {
			out.CapacityKnown, out.Max, out.UtilizationPct = false, 0, 0
			out.Reason = err.Error()
			return out, nil
		}
		if out.Max > 0 {
			out.UtilizationPct = float64(out.Total) * 100 / float64(out.Max)
		}
		return out, err
	})
}

func TableStructure(ctx context.Context, in ConnectionInput, input TableColumnsInput) (TableStructureOutput, error) {
	input.Schema, input.Table = strings.TrimSpace(input.Schema), strings.TrimSpace(input.Table)
	if input.Schema == "" || input.Table == "" {
		return TableStructureOutput{}, errors.New("schema and table are required")
	}
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (TableStructureOutput, error) {
		out := TableStructureOutput{Schema: input.Schema, Table: input.Table, Constraints: []relational.TableConstraint{}, Indexes: []relational.TableIndex{}}
		var ignored, ddl string
		if err := db.QueryRowContext(ctx, "SHOW CREATE TABLE `"+strings.ReplaceAll(input.Schema, "`", "``")+"`.`"+strings.ReplaceAll(input.Table, "`", "``")+"`").Scan(&ignored, &ddl); err != nil {
			return out, err
		}
		out.CreateSQL = ddl
		rows, err := db.QueryContext(ctx, `SELECT tc.CONSTRAINT_NAME,tc.CONSTRAINT_TYPE,COALESCE(GROUP_CONCAT(kcu.COLUMN_NAME ORDER BY kcu.ORDINAL_POSITION),''),'','','' FROM information_schema.TABLE_CONSTRAINTS tc LEFT JOIN information_schema.KEY_COLUMN_USAGE kcu ON kcu.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND kcu.TABLE_NAME=tc.TABLE_NAME AND kcu.CONSTRAINT_NAME=tc.CONSTRAINT_NAME WHERE tc.TABLE_SCHEMA=? AND tc.TABLE_NAME=? GROUP BY tc.CONSTRAINT_NAME,tc.CONSTRAINT_TYPE ORDER BY tc.CONSTRAINT_NAME`, input.Schema, input.Table)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var c relational.TableConstraint
			if err := rows.Scan(&c.Name, &c.Type, &c.Columns, &c.Definition, &c.Status, &c.Validated); err != nil {
				rows.Close()
				return out, err
			}
			out.Constraints = append(out.Constraints, c)
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return out, err
		}
		rows.Close()
		rows, err = db.QueryContext(ctx, `SELECT INDEX_NAME,INDEX_TYPE,COALESCE(GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX),''),'',0,INDEX_NAME='PRIMARY',MAX(NON_UNIQUE=0),'visible' FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? GROUP BY INDEX_NAME,INDEX_TYPE ORDER BY INDEX_NAME`, input.Schema, input.Table)
		if err != nil {
			return out, err
		}
		defer rows.Close()
		for rows.Next() {
			var idx relational.TableIndex
			if err := rows.Scan(&idx.Name, &idx.Type, &idx.Columns, &idx.Definition, &idx.SizeBytes, &idx.IsPrimary, &idx.IsUnique, &idx.Status); err != nil {
				return out, err
			}
			out.Indexes = append(out.Indexes, idx)
		}
		return out, rows.Err()
	})
}
