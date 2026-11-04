# Relational Database Diagnostics

PostgreSQL, MySQL, Oracle and OceanBase expose the same read-only diagnostic tools and JSON result types through Direct and MCP: slow queries, active queries, lock waits, connection statistics and table structure. The shared wire types live in `backend/tool/relational`.

The contract standardizes field names and meanings where possible; it does not claim that engines collect identical measurements or retention windows:

| Tool | Common result | Engine-specific measurement |
| --- | --- | --- |
| `*_slow_queries` | Query ID, observed executions, total/mean/max execution milliseconds, rows and SQL text | PostgreSQL uses `pg_stat_statements`; MySQL uses Performance Schema digest summaries; Oracle uses the shared cursor cache; OceanBase aggregates retained SQL Audit records whose request start falls in the last 30 minutes. These sources have different reset, retention and execution-count semantics. |
| `*_active_queries` | Session ID, user, client, state, wait fields, start time, duration and SQL | Wait fields unavailable from an engine's active-session source are empty. Session IDs are engine-local session identifiers, not operating-system PIDs. |
| `*_lock_waits` | Blocked/blocking session and user, SQL, wait duration and optional lock identity | `wait_duration_known` distinguishes an actual duration from an unavailable measurement. An engine or account that cannot expose blocker details returns `available: false` and a reason rather than an empty list that looks like no waits. |
| `*_connection_stats` | Total, active, idle, idle-in-transaction, maximum and utilization | `capacity_known` is false when the engine does not expose a comparable maximum. In that case `max` and utilization remain zero/unknown. |
| `*_table_structure` | Schema/table, DDL when available, constraints and indexes | Engines without a safe/readable DDL source leave `create_sql` empty while returning metadata they can read. |

OceanBase is not implemented by reusing MySQL or Oracle diagnostics SQL. Its MySQL-compatible connection uses OceanBase system views under `oceanbase.GV$OB_*`; its Oracle-compatible connection uses `SYS.GV$OB_*`. SQL Audit, tenant process-list and lock-view support varies by OceanBase version and account grants. SQL Audit summarizes retained request records whose start time falls in the last 30 minutes. Lock wait duration is converted from the native microsecond `CTIME` to seconds. Unsupported views or missing privileges return `available: false` and the server error; no MySQL Performance Schema or Oracle `V$SESSION` fallback is assumed. OceanBase connection counts are available independently of maximum-capacity reporting.

All tools are read-only and bounded. SQL text is truncated to 4 KiB, and result lists are capped at 100 rows (slow-query limit is configurable from 1 to 100). Availability flags distinguish an empty result from unavailable views or missing privileges.
