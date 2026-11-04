package oracle

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
		rows, err := db.QueryContext(ctx, `SELECT SQL_ID,EXECUTIONS,ELAPSED_TIME/1000,CASE WHEN EXECUTIONS>0 THEN ELAPSED_TIME/1000/EXECUTIONS ELSE 0 END,0,ROWS_PROCESSED,SUBSTR(SQL_TEXT,1,4096) FROM (SELECT SQL_ID,EXECUTIONS,ELAPSED_TIME,ROWS_PROCESSED,SQL_TEXT FROM V$SQLAREA WHERE PARSING_SCHEMA_NAME=USER AND EXECUTIONS>0 AND ELAPSED_TIME/1000>=:1 ORDER BY ELAPSED_TIME DESC) WHERE ROWNUM<=:2`, input.MinTotalMS, input.Limit)
		if err != nil {
			out.Available = false
			out.Reason = err.Error()
			return out, nil
		}
		defer rows.Close()
		for rows.Next() {
			var q relational.SlowQuery
			if err := rows.Scan(&q.QueryID, &q.Calls, &q.TotalExecMS, &q.MeanExecMS, &q.MaxExecMS, &q.Rows, &q.Query); err != nil {
				return out, err
			}
			out.Queries = append(out.Queries, q)
		}
		return out, rows.Err()
	})
}

func ActiveQueries(ctx context.Context, in ConnectionInput) (ActiveQueriesOutput, error) {
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (ActiveQueriesOutput, error) {
		out := ActiveQueriesOutput{Available: true, Queries: []relational.ActiveQuery{}}
		rows, err := db.QueryContext(ctx, `SELECT s.SID,COALESCE(s.USERNAME,''),COALESCE(s.PROGRAM,''),COALESCE(s.MACHINE,''),COALESCE(s.STATUS,''),COALESCE(s.WAIT_CLASS,''),COALESCE(s.EVENT,''),COALESCE(TO_CHAR(s.SQL_EXEC_START,'YYYY-MM-DD HH24:MI:SS'),''),COALESCE(s.LAST_CALL_ET,0),SUBSTR(COALESCE(q.SQL_TEXT,''),1,4096) FROM V$SESSION s LEFT JOIN V$SQL q ON q.SQL_ID=s.SQL_ID AND q.CHILD_NUMBER=s.SQL_CHILD_NUMBER WHERE s.TYPE='USER' AND s.USERNAME=USER AND s.STATUS='ACTIVE' AND s.SID<>TO_NUMBER(SYS_CONTEXT('USERENV','SID')) ORDER BY s.LAST_CALL_ET DESC FETCH FIRST 100 ROWS ONLY`)
		if err != nil {
			out.Available, out.Reason = false, err.Error()
			return out, nil
		}
		defer rows.Close()
		for rows.Next() {
			var q relational.ActiveQuery
			if err := rows.Scan(&q.SessionID, &q.User, &q.ApplicationName, &q.ClientAddress, &q.State, &q.WaitEventType, &q.WaitEvent, &q.QueryStart, &q.DurationSeconds, &q.Query); err != nil {
				return out, err
			}
			out.Queries = append(out.Queries, q)
		}
		return out, rows.Err()
	})
}

func LockWaits(ctx context.Context, in ConnectionInput) (LockWaitsOutput, error) {
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (LockWaitsOutput, error) {
		out := LockWaitsOutput{Available: true, Waits: []relational.LockWait{}}
		rows, err := db.QueryContext(ctx, `SELECT a.SID,b.SID,COALESCE(a.USERNAME,''),COALESCE(b.USERNAME,''),SUBSTR(COALESCE(qa.SQL_TEXT,''),1,4096),SUBSTR(COALESCE(qb.SQL_TEXT,''),1,4096),COALESCE(a.SECONDS_IN_WAIT,0),1,'','' FROM V$SESSION a JOIN V$SESSION b ON b.SID=a.BLOCKING_SESSION LEFT JOIN V$SQL qa ON qa.SQL_ID=a.SQL_ID AND qa.CHILD_NUMBER=a.SQL_CHILD_NUMBER LEFT JOIN V$SQL qb ON qb.SQL_ID=b.SQL_ID AND qb.CHILD_NUMBER=b.SQL_CHILD_NUMBER WHERE a.BLOCKING_SESSION IS NOT NULL AND a.TYPE='USER' ORDER BY a.SECONDS_IN_WAIT DESC FETCH FIRST 100 ROWS ONLY`)
		if err != nil {
			out.Available = false
			out.Reason = err.Error()
			return out, nil
		}
		defer rows.Close()
		for rows.Next() {
			var w relational.LockWait
			if err := rows.Scan(&w.BlockedSessionID, &w.BlockingSessionID, &w.BlockedUser, &w.BlockingUser, &w.BlockedQuery, &w.BlockingQuery, &w.WaitDurationSeconds, &w.WaitDurationKnown, &w.LockType, &w.LockResource); err != nil {
				return out, err
			}
			out.Waits = append(out.Waits, w)
		}
		return out, rows.Err()
	})
}

func ConnectionStats(ctx context.Context, in ConnectionInput) (ConnectionStatsOutput, error) {
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB) (ConnectionStatsOutput, error) {
		out := ConnectionStatsOutput{Available: true, CapacityKnown: true}
		err := db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(CASE WHEN s.STATUS='ACTIVE' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN s.STATUS<>'ACTIVE' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN s.STATUS<>'ACTIVE' AND t.ADDR IS NOT NULL THEN 1 ELSE 0 END),0) FROM V$SESSION s LEFT JOIN V$TRANSACTION t ON t.ADDR=s.TADDR WHERE s.TYPE='USER' AND s.USERNAME=USER AND s.SID<>TO_NUMBER(SYS_CONTEXT('USERENV','SID'))`).Scan(&out.Total, &out.Active, &out.Idle, &out.IdleInTransaction)
		if err != nil {
			return out, err
		}
		err = db.QueryRowContext(ctx, `SELECT TO_NUMBER(VALUE) FROM V$PARAMETER WHERE NAME='sessions'`).Scan(&out.Max)
		if err != nil {
			out.CapacityKnown, out.Reason = false, err.Error()
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
		owner, table := strings.ToUpper(input.Schema), strings.ToUpper(input.Table)
		rows, err := db.QueryContext(ctx, `SELECT c.CONSTRAINT_NAME,c.CONSTRAINT_TYPE,COALESCE(LISTAGG(cc.COLUMN_NAME,', ') WITHIN GROUP (ORDER BY cc.POSITION),''),'',c.STATUS,c.VALIDATED FROM ALL_CONSTRAINTS c LEFT JOIN ALL_CONS_COLUMNS cc ON cc.OWNER=c.OWNER AND cc.TABLE_NAME=c.TABLE_NAME AND cc.CONSTRAINT_NAME=c.CONSTRAINT_NAME WHERE c.OWNER=:1 AND c.TABLE_NAME=:2 GROUP BY c.CONSTRAINT_NAME,c.CONSTRAINT_TYPE,c.STATUS,c.VALIDATED ORDER BY c.CONSTRAINT_NAME`, owner, table)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var item relational.TableConstraint
			if err := rows.Scan(&item.Name, &item.Type, &item.Columns, &item.Definition, &item.Status, &item.Validated); err != nil {
				rows.Close()
				return out, err
			}
			out.Constraints = append(out.Constraints, item)
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return out, err
		}
		rows.Close()
		rows, err = db.QueryContext(ctx, `SELECT i.INDEX_NAME,i.INDEX_TYPE,COALESCE(LISTAGG(ic.COLUMN_NAME,', ') WITHIN GROUP (ORDER BY ic.COLUMN_POSITION),''),'',COALESCE(s.BYTES,0),CASE WHEN EXISTS (SELECT 1 FROM ALL_CONSTRAINTS c WHERE c.OWNER=i.TABLE_OWNER AND c.TABLE_NAME=i.TABLE_NAME AND c.INDEX_NAME=i.INDEX_NAME AND c.CONSTRAINT_TYPE='P') THEN 1 ELSE 0 END,CASE WHEN i.UNIQUENESS='UNIQUE' THEN 1 ELSE 0 END,i.STATUS FROM ALL_INDEXES i LEFT JOIN ALL_IND_COLUMNS ic ON ic.INDEX_OWNER=i.OWNER AND ic.INDEX_NAME=i.INDEX_NAME AND ic.TABLE_OWNER=i.TABLE_OWNER AND ic.TABLE_NAME=i.TABLE_NAME LEFT JOIN ALL_SEGMENTS s ON s.OWNER=i.OWNER AND s.SEGMENT_NAME=i.INDEX_NAME AND s.SEGMENT_TYPE='INDEX' WHERE i.TABLE_OWNER=:1 AND i.TABLE_NAME=:2 GROUP BY i.INDEX_NAME,i.INDEX_TYPE,s.BYTES,i.TABLE_OWNER,i.TABLE_NAME,i.UNIQUENESS,i.STATUS ORDER BY i.INDEX_NAME`, owner, table)
		if err != nil {
			return out, err
		}
		defer rows.Close()
		for rows.Next() {
			var item relational.TableIndex
			var primary, unique int
			if err := rows.Scan(&item.Name, &item.Type, &item.Columns, &item.Definition, &item.SizeBytes, &primary, &unique, &item.Status); err != nil {
				return out, err
			}
			item.IsPrimary = primary != 0
			item.IsUnique = unique != 0
			out.Indexes = append(out.Indexes, item)
		}
		return out, rows.Err()
	})
}
