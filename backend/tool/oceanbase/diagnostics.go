package oceanbase

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strings"
	"time"

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
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB, m, route string) (SlowQueriesOutput, error) {
		out := SlowQueriesOutput{Available: true, Queries: []relational.SlowQuery{}}
		schema, database := "oceanbase", "DATABASE()"
		cutoff, min, limit := "?", "?", "LIMIT 100"
		queryText := `MAX(SUBSTR(QUERY_SQL,1,4096))`
		if m == ModeOracle {
			schema, database = "SYS", `SYS_CONTEXT('USERENV','CURRENT_SCHEMA')`
			queryText = `MAX(DBMS_LOB.SUBSTR(QUERY_SQL,4000,1))`
			if route == "oracle-tns" {
				cutoff, min, limit = ":1", ":2", "FETCH FIRST 100 ROWS ONLY"
			} else {
				cutoff, min, limit = "?", "?", "FETCH FIRST 100 ROWS ONLY"
			}
		}
		q := `SELECT SQL_ID,COUNT(*),SUM(ELAPSED_TIME)/1000,AVG(ELAPSED_TIME)/1000,MAX(ELAPSED_TIME)/1000,SUM(RETURN_ROWS),` + queryText + ` FROM ` + schema + `.GV$OB_SQL_AUDIT WHERE IS_INNER_SQL=0 AND IS_EXECUTOR_RPC=0 AND SQL_ID IS NOT NULL AND SQL_ID<>'' AND DB_NAME=` + database + ` AND REQUEST_TIME>` + cutoff + ` GROUP BY SQL_ID HAVING SUM(ELAPSED_TIME)>=` + min + `*1000 ORDER BY SUM(ELAPSED_TIME) DESC ` + limit
		rows, err := db.QueryContext(ctx, q, time.Now().Add(-30*time.Minute).UnixMicro(), input.MinTotalMS)
		if err != nil {
			out.Available = false
			out.Reason = err.Error()
			return out, nil
		}
		defer rows.Close()
		for rows.Next() {
			if len(out.Queries) >= input.Limit {
				break
			}
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
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB, m, _ string) (ActiveQueriesOutput, error) {
		out := ActiveQueriesOutput{Available: true, Queries: []relational.ActiveQuery{}}
		schema, duration, limit, currentSession := "oceanbase", "TIME", "LIMIT 100", "CONNECTION_ID()"
		if m == ModeOracle {
			schema, duration, limit, currentSession = "SYS", "TIME", "FETCH FIRST 100 ROWS ONLY", `TO_NUMBER(SYS_CONTEXT('USERENV','SID'))`
		}
		q := `SELECT ID,COALESCE(USER,''),'',COALESCE(HOST,''),COALESCE(STATE,''),'','','',COALESCE(` + duration + `,0),SUBSTR(COALESCE(INFO,''),1,4096) FROM ` + schema + `.GV$OB_PROCESSLIST WHERE COMMAND<>'Sleep' AND NOT (SVR_IP=HOST_IP() AND SVR_PORT=RPC_PORT() AND ID=` + currentSession + `) ORDER BY TIME DESC ` + limit
		rows, err := db.QueryContext(ctx, q)
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
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB, m, _ string) (LockWaitsOutput, error) {
		out := LockWaitsOutput{Available: true, Waits: []relational.LockWait{}}
		schema, limit, resource, waitSeconds := "oceanbase", "LIMIT 100", `CONCAT(w.TYPE,':',w.ID1,':',COALESCE(w.ID2,''))`, "FLOOR(w.CTIME/1000000)"
		if m == ModeOracle {
			schema, limit, resource, waitSeconds = "SYS", "FETCH FIRST 100 ROWS ONLY", `w.TYPE||':'||TO_CHAR(w.ID1)||':'||NVL(TO_CHAR(w.ID2),'')`, "FLOOR(w.CTIME/1000000)"
		}
		q := `SELECT DISTINCT wp.ID,bp.ID,COALESCE(wp.USER,''),COALESCE(bp.USER,''),SUBSTR(COALESCE(wp.INFO,''),1,4096),SUBSTR(COALESCE(bp.INFO,''),1,4096),` + waitSeconds + `,1,w.TYPE,` + resource + ` FROM ` + schema + `.GV$OB_LOCKS w JOIN ` + schema + `.GV$OB_LOCKS h ON h.TENANT_ID=w.TENANT_ID AND h.TYPE=w.TYPE AND h.TRANS_ID=w.ID1 AND h.LMODE<>'NONE' AND h.BLOCK=0 JOIN ` + schema + `.GV$OB_PROCESSLIST wp ON wp.ID=w.SESSION_ID AND wp.SVR_IP=w.SVR_IP AND wp.SVR_PORT=w.SVR_PORT JOIN ` + schema + `.GV$OB_PROCESSLIST bp ON bp.ID=h.SESSION_ID AND bp.SVR_IP=h.SVR_IP AND bp.SVR_PORT=h.SVR_PORT WHERE w.BLOCK=1 AND w.REQUEST<>'NONE' ORDER BY ` + waitSeconds + ` DESC ` + limit
		rows, err := db.QueryContext(ctx, q)
		if err != nil {
			out.Available = false
			out.Reason = err.Error()
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
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB, m, _ string) (ConnectionStatsOutput, error) {
		var out ConnectionStatsOutput
		out.Available = true
		out.CapacityKnown = false
		schema, currentSession := "oceanbase", "CONNECTION_ID()"
		if m == ModeOracle {
			schema, currentSession = "SYS", `TO_NUMBER(SYS_CONTEXT('USERENV','SID'))`
		}
		err := db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(CASE WHEN p.COMMAND<>'Sleep' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN p.COMMAND='Sleep' AND COALESCE(p.TRANS_STATE,'IDLE') NOT IN ('ACTIVE','IMPLICIT_ACTIVE') THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN p.COMMAND='Sleep' AND p.TRANS_STATE IN ('ACTIVE','IMPLICIT_ACTIVE') THEN 1 ELSE 0 END),0) FROM `+schema+`.GV$OB_PROCESSLIST p WHERE NOT (p.SVR_IP=HOST_IP() AND p.SVR_PORT=RPC_PORT() AND p.ID=`+currentSession+`)`).Scan(&out.Total, &out.Active, &out.Idle, &out.IdleInTransaction)
		if err != nil {
			out.Available, out.Reason = false, err.Error()
			return out, nil
		}
		return out, nil
	})
}

func TableStructure(ctx context.Context, in ConnectionInput, input TableColumnsInput) (TableStructureOutput, error) {
	input.Schema, input.Table = strings.TrimSpace(input.Schema), strings.TrimSpace(input.Table)
	if input.Schema == "" || input.Table == "" {
		return TableStructureOutput{}, errors.New("schema and table are required")
	}
	return withDB(ctx, in, func(ctx context.Context, db *sql.DB, m, route string) (TableStructureOutput, error) {
		out := TableStructureOutput{Schema: input.Schema, Table: input.Table, Constraints: []relational.TableConstraint{}, Indexes: []relational.TableIndex{}}
		if m == ModeMySQL {
			var ignored string
			if err := db.QueryRowContext(ctx, "SHOW CREATE TABLE `"+strings.ReplaceAll(input.Schema, "`", "``")+"`.`"+strings.ReplaceAll(input.Table, "`", "``")+"`").Scan(&ignored, &out.CreateSQL); err != nil {
				return out, err
			}
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
		}
		owner, table := strings.ToUpper(input.Schema), strings.ToUpper(input.Table)
		ownerBind, tableBind := ":1", ":2"
		if route == "obclient-mysql-wire" {
			ownerBind, tableBind = "?", "?"
		}
		rows, err := db.QueryContext(ctx, `SELECT c.CONSTRAINT_NAME,c.CONSTRAINT_TYPE,COALESCE(LISTAGG(cc.COLUMN_NAME,', ') WITHIN GROUP (ORDER BY cc.POSITION),''),'',c.STATUS,c.VALIDATED FROM ALL_CONSTRAINTS c LEFT JOIN ALL_CONS_COLUMNS cc ON cc.OWNER=c.OWNER AND cc.TABLE_NAME=c.TABLE_NAME AND cc.CONSTRAINT_NAME=c.CONSTRAINT_NAME WHERE c.OWNER=`+ownerBind+` AND c.TABLE_NAME=`+tableBind+` GROUP BY c.CONSTRAINT_NAME,c.CONSTRAINT_TYPE,c.STATUS,c.VALIDATED ORDER BY c.CONSTRAINT_NAME`, owner, table)
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
		rows, err = db.QueryContext(ctx, `SELECT i.INDEX_NAME,i.INDEX_TYPE,COALESCE(LISTAGG(ic.COLUMN_NAME,', ') WITHIN GROUP (ORDER BY ic.COLUMN_POSITION),''),'',COALESCE(s.BYTES,0),CASE WHEN EXISTS (SELECT 1 FROM ALL_CONSTRAINTS c WHERE c.OWNER=i.TABLE_OWNER AND c.TABLE_NAME=i.TABLE_NAME AND c.INDEX_NAME=i.INDEX_NAME AND c.CONSTRAINT_TYPE='P') THEN 1 ELSE 0 END,CASE WHEN i.UNIQUENESS='UNIQUE' THEN 1 ELSE 0 END,i.STATUS FROM ALL_INDEXES i LEFT JOIN ALL_IND_COLUMNS ic ON ic.INDEX_OWNER=i.OWNER AND ic.INDEX_NAME=i.INDEX_NAME AND ic.TABLE_OWNER=i.TABLE_OWNER AND ic.TABLE_NAME=i.TABLE_NAME LEFT JOIN ALL_SEGMENTS s ON s.OWNER=i.OWNER AND s.SEGMENT_NAME=i.INDEX_NAME AND s.SEGMENT_TYPE='INDEX' WHERE i.TABLE_OWNER=`+ownerBind+` AND i.TABLE_NAME=`+tableBind+` GROUP BY i.INDEX_NAME,i.INDEX_TYPE,s.BYTES,i.TABLE_OWNER,i.TABLE_NAME,i.UNIQUENESS,i.STATUS ORDER BY i.INDEX_NAME`, owner, table)
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
