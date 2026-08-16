package connector

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

type postgreSQLAdapter struct {
	config *pgx.ConnConfig
}

func newPostgreSQLAdapter(target Target, _ Limits) (Adapter, error) {
	host := configString(target.Resource.Config, "host")
	database := configString(target.Resource.Config, "database")
	if host == "" || database == "" {
		return nil, connectorError(CategoryConfiguration, "configure PostgreSQL", false, errors.New("host and database are required"))
	}
	port := configPort(target.Resource.Config, 5432)
	secret := secretFields(target.Secret)
	username := strings.TrimSpace(secret["username"])
	password := secret["password"]
	if username == "" || password == "" {
		return nil, connectorError(CategoryConfiguration, "configure PostgreSQL", false, errors.New("credential must contain username and password"))
	}
	connectionURL := (&url.URL{Scheme: "postgres", User: url.UserPassword(username, password), Host: net.JoinHostPort(host, strconv.Itoa(port)), Path: database, RawQuery: "sslmode=prefer"}).String()
	config, err := pgx.ParseConfig(connectionURL)
	if err != nil {
		return nil, connectorError(CategoryConfiguration, "parse PostgreSQL connection", false, err)
	}
	config.RuntimeParams["default_transaction_read_only"] = "on"
	config.RuntimeParams["statement_timeout"] = "8000"
	config.RuntimeParams["application_name"] = "opskeeper-diagnostic"
	return &postgreSQLAdapter{config: config}, nil
}

func (a *postgreSQLAdapter) Kind() string { return "PostgreSQL" }
func (a *postgreSQLAdapter) Capabilities() []Capability {
	return []Capability{CapabilityPostgreSQLInspect}
}
func (a *postgreSQLAdapter) Test(ctx context.Context) error {
	conn, err := pgx.ConnectConfig(ctx, a.config)
	if err != nil {
		return postgreSQLError("connect PostgreSQL", err)
	}
	defer conn.Close(ctx)
	return postgreSQLError("ping PostgreSQL", conn.Ping(ctx))
}

func (a *postgreSQLAdapter) InspectPostgreSQL(ctx context.Context) (DiagnosticSnapshot, error) {
	conn, err := pgx.ConnectConfig(ctx, a.config)
	if err != nil {
		return DiagnosticSnapshot{}, postgreSQLError("connect PostgreSQL", err)
	}
	defer conn.Close(ctx)
	if err := conn.Ping(ctx); err != nil {
		return DiagnosticSnapshot{}, postgreSQLError("ping PostgreSQL", err)
	}
	snapshot := DiagnosticSnapshot{Kind: "PostgreSQL", Facts: map[string]any{}, Findings: []Finding{}, Capabilities: []string{"connection", "sessions", "slow_queries", "locks", "replication", "capacity"}, Unavailable: []string{}}
	var version string
	if err := conn.QueryRow(ctx, "SHOW server_version").Scan(&version); err != nil {
		return DiagnosticSnapshot{}, postgreSQLError("read PostgreSQL version", err)
	}
	snapshot.Facts["server_version"] = version
	var activeSessions int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE state = 'active'").Scan(&activeSessions); err != nil {
		snapshot.Unavailable = append(snapshot.Unavailable, "sessions")
	} else {
		snapshot.Facts["active_sessions"] = activeSessions
	}
	var longRunningQueries int
	if err := conn.QueryRow(ctx, `
		SELECT count(*)
		  FROM pg_stat_activity
		 WHERE state = 'active'
		   AND pid <> pg_backend_pid()
		   AND query_start < clock_timestamp() - interval '5 seconds'`).Scan(&longRunningQueries); err != nil {
		snapshot.Unavailable = append(snapshot.Unavailable, "slow_queries")
	} else {
		snapshot.Facts["long_running_queries"] = longRunningQueries
		if longRunningQueries > 0 {
			snapshot.Findings = append(snapshot.Findings, Finding{Code: "postgresql.long_running_queries", Severity: "warning", Message: "存在执行超过 5 秒的活跃查询"})
		}
	}
	var waitingLocks int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM pg_locks WHERE NOT granted").Scan(&waitingLocks); err != nil {
		snapshot.Unavailable = append(snapshot.Unavailable, "locks")
	} else {
		snapshot.Facts["waiting_locks"] = waitingLocks
		if waitingLocks > 0 {
			snapshot.Findings = append(snapshot.Findings, Finding{Code: "postgresql.waiting_locks", Severity: "warning", Message: "存在等待中的数据库锁"})
		}
	}
	var replicas int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM pg_stat_replication").Scan(&replicas); err != nil {
		snapshot.Unavailable = append(snapshot.Unavailable, "replication")
	} else {
		snapshot.Facts["replica_count"] = replicas
	}
	var size int64
	if err := conn.QueryRow(ctx, "SELECT pg_database_size(current_database())").Scan(&size); err != nil {
		snapshot.Unavailable = append(snapshot.Unavailable, "capacity")
	} else {
		snapshot.Facts["database_size_bytes"] = size
	}
	return snapshot, nil
}

func configPort(config map[string]any, fallback int) int {
	if value, ok := config["port"].(float64); ok && value > 0 && value <= 65535 {
		return int(value)
	}
	if value, ok := config["port"].(int); ok && value > 0 && value <= 65535 {
		return value
	}
	return fallback
}
func postgreSQLError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return connectorError(CategoryTimeout, operation, true, err)
	}
	return connectorError(CategoryUpstream, operation, true, err)
}
