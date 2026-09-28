package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/config"
	"opskeeper/backend/identity"
)

const (
	markerKey   = "opskeeper.dev-sample"
	markerValue = "access-v1"
	password    = "Sample123!"
	emailDomain = "yunops.test"
)

var sampleUsernames = []string{
	"zhangwei",
	"lina",
	"wangtao",
	"chenyu",
	"zhaolei",
	"sunmin",
}

var legacySampleUsernames = []string{
	"sample_platform",
	"sample_team_admin",
	"sample_team_ops",
	"sample_project_mix",
	"sample_project_gamma",
	"sample_project_viewer",
}

type team struct {
	key      string
	name     string
	icon     string
	projects []project
}

type project struct {
	name string
	code string
}

type user struct {
	username string
	name     string
	roles    []binding
}

type binding struct {
	role  string
	scope string
}

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "seed" && os.Args[1] != "clean") {
		fmt.Fprintln(os.Stderr, "usage: sampledata seed|clean")
		os.Exit(2)
	}

	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(action string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	if err := ensureLocalDatabase(cfg.Environment, cfg.DatabaseURL); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("configure database: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connect to local PostgreSQL: %w", err)
	}

	if action == "seed" {
		return seed(ctx, pool)
	}
	return clean(ctx, pool)
}

func ensureLocalDatabase(environment, databaseURL string) error {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return fmt.Errorf("parse database URL: %w", err)
	}
	host := parsed.Hostname()
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return errors.New("sample data commands require a local PostgreSQL database")
	}
	if environment != "development" || !isLoopback(host) || strings.TrimPrefix(parsed.Path, "/") != "opskeeper" {
		return errors.New("refusing sample data operation: only development database opskeeper on localhost is allowed")
	}
	return nil
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func seed(ctx context.Context, pool *pgxpool.Pool) error {
	hash, err := identity.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash sample password: %w", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin sample seed: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, markerKey+":"+markerValue); err != nil {
		return fmt.Errorf("lock sample seed: %w", err)
	}

	var existing bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM teams WHERE labels->>$1 = $2 AND deleted_at IS NULL
			UNION ALL
			SELECT 1 FROM projects WHERE labels->>$1 = $2 AND deleted_at IS NULL
			UNION ALL
			SELECT 1 FROM users WHERE username = ANY($3::text[]) AND deleted_at IS NULL
			UNION ALL
			SELECT 1 FROM users WHERE username = ANY($4::text[]) AND deleted_at IS NULL
		)`, markerKey, markerValue, sampleUsernames, legacySampleUsernames).Scan(&existing); err != nil {
		return fmt.Errorf("check existing sample data: %w", err)
	}
	if existing {
		return errors.New("sample data already exists; run `make sample-clean` first")
	}

	var platformID, platformScopeID, tenantID string
	if err := tx.QueryRow(ctx, `
		SELECT p.id::text, s.id::text, s.tenant_id
		  FROM platforms p JOIN scopes s ON s.id = p.scope_id
		 WHERE p.code = 'default' AND p.deleted_at IS NULL AND s.deleted_at IS NULL`).
		Scan(&platformID, &platformScopeID, &tenantID); err != nil {
		return fmt.Errorf("find default platform: %w", err)
	}

	teams := []team{
		{key: "alpha", name: "云平台研发部", icon: "lucide:Code2", projects: []project{{"统一身份中心", "iam001"}, {"API 网关", "api001"}, {"服务目录平台", "svc001"}, {"配置管理中心", "cfg001"}, {"消息通知服务", "msg001"}, {"研发效能门户", "dev001"}, {"订单履约系统", "ord001"}, {"设备接入平台", "iot001"}, {"审计日志中心", "aud001"}, {"持续交付平台", "cic001"}, {"运维工单系统", "ops001"}, {"服务注册中心", "reg001"}}},
		{key: "beta", name: "基础设施运维部", icon: "lucide:Server", projects: []project{{"容器平台管理", "k8s001"}, {"主机监控告警", "mon001"}}},
		{key: "gamma", name: "数据智能部", icon: "lucide:Database", projects: []project{{"数据集成平台", "etl001"}, {"实时指标分析", "bi001"}}},
	}
	teamScopes := make(map[string]string, len(teams))
	projects := make(map[string]string)
	for _, item := range teams {
		var scopeID, teamID string
		if err := tx.QueryRow(ctx, `INSERT INTO scopes (tenant_id, scope_type, parent_scope_id) VALUES ($1, 'team', $2::uuid) RETURNING id::text`, tenantID, platformScopeID).Scan(&scopeID); err != nil {
			return fmt.Errorf("create team scope %s: %w", item.key, err)
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO teams (scope_id, platform_id, name, description, icon, labels)
			VALUES ($1::uuid, $2::uuid, $3, 'OpsKeeper 权限演示数据', $4, jsonb_build_object($5::text, $6::text))
			RETURNING id::text`, scopeID, platformID, item.name, item.icon, markerKey, markerValue).Scan(&teamID); err != nil {
			return fmt.Errorf("create sample team %s: %w", item.key, err)
		}
		teamScopes[item.key] = scopeID
		for index, projectItem := range item.projects {
			code := projectItem.code
			var projectScopeID, projectID string
			if err := tx.QueryRow(ctx, `INSERT INTO scopes (tenant_id, scope_type, parent_scope_id) VALUES ($1, 'project', $2::uuid) RETURNING id::text`, tenantID, scopeID).Scan(&projectScopeID); err != nil {
				return fmt.Errorf("create project scope %s: %w", code, err)
			}
			if err := tx.QueryRow(ctx, `
				INSERT INTO projects (scope_id, platform_id, team_id, name, code, icon, labels)
				VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, 'lucide:FolderKanban', jsonb_build_object($6::text, $7::text))
				RETURNING id::text`, projectScopeID, platformID, teamID, projectItem.name, code, markerKey, markerValue).Scan(&projectID); err != nil {
				return fmt.Errorf("create sample project %s: %w", code, err)
			}
			projects[item.key+fmt.Sprintf("-%02d", index)] = projectScopeID
		}
	}

	users := []user{
		{username: "zhangwei", name: "张伟", roles: []binding{{"PlatformViewer", "platform"}, {"TeamViewer", "alpha"}, {"ProjectViewer", "beta-01"}}},
		{username: "lina", name: "李娜", roles: []binding{{"TeamAdmin", "alpha"}, {"ProjectViewer", "alpha-01"}}},
		{username: "wangtao", name: "王涛", roles: []binding{{"TeamOperator", "beta"}, {"ProjectOperator", "beta-02"}}},
		{username: "chenyu", name: "陈宇", roles: []binding{{"TeamViewer", "gamma"}, {"ProjectAdmin", "gamma-01"}}},
		{username: "zhaolei", name: "赵磊", roles: []binding{{"TeamOperator", "alpha"}, {"TeamViewer", "gamma"}, {"ProjectViewer", "beta-01"}}},
		{username: "sunmin", name: "孙敏", roles: []binding{{"ProjectAdmin", "alpha-01"}, {"ProjectViewer", "alpha-02"}, {"ProjectViewer", "gamma-02"}}},
	}
	userIDs := make(map[string]string, len(users))
	for _, item := range users {
		var userID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO users (username, email, display_name)
			VALUES ($1, $2, $3) RETURNING id::text`, item.username, item.username+"@"+emailDomain, item.name).Scan(&userID); err != nil {
			return fmt.Errorf("create sample user %s: %w", item.username, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO credentials (user_id, password_hash, must_change_password) VALUES ($1::uuid, $2, false)`, userID, hash); err != nil {
			return fmt.Errorf("create credentials for %s: %w", item.username, err)
		}
		userIDs[item.username] = userID
	}

	for _, item := range users {
		for _, grant := range item.roles {
			scopeID := platformScopeID
			if grant.scope != "platform" {
				var ok bool
				scopeID, ok = teamScopes[grant.scope]
				if !ok {
					scopeID, ok = projects[grant.scope]
				}
				if !ok {
					return fmt.Errorf("unknown sample scope %q", grant.scope)
				}
			}
			command, err := tx.Exec(ctx, `
				INSERT INTO role_bindings (subject_type, subject_id, role_id, scope_id)
				SELECT 'user', $1::uuid, role.id, $2::uuid FROM roles role
				 WHERE role.name = $3 AND role.scope_type = CASE WHEN $4 = 'platform' THEN 'platform' WHEN $4 IN ('alpha', 'beta', 'gamma') THEN 'team' ELSE 'project' END`,
				userIDs[item.username], scopeID, grant.role, grant.scope)
			if err != nil {
				return fmt.Errorf("bind %s to %s at %s: %w", item.username, grant.role, grant.scope, err)
			}
			if command.RowsAffected() != 1 {
				return fmt.Errorf("role %s was not found for scope %s", grant.role, grant.scope)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit sample seed: %w", err)
	}
	fmt.Printf("Seeded 3 teams, 16 projects, and 6 users. Sample login password: %s\n", password)
	for _, item := range users {
		fmt.Printf("  %-24s %s@%s\n", item.username, item.username, emailDomain)
	}
	return nil
}

func clean(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin sample cleanup: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, markerKey+":"+markerValue); err != nil {
		return fmt.Errorf("lock sample cleanup: %w", err)
	}

	var userIDs, teamIDs, teamScopeIDs, projectIDs, projectScopeIDs []string
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(array_agg(id::text), ARRAY[]::text[])
		  FROM users
		 WHERE (username = ANY($1::text[]) AND email LIKE $2)
		    OR (username = ANY($3::text[]) AND email LIKE $4)`,
		sampleUsernames, "%@"+emailDomain, legacySampleUsernames, "%@sample.opskeeper.invalid").Scan(&userIDs); err != nil {
		return fmt.Errorf("find sample users: %w", err)
	}
	if err := tx.QueryRow(ctx, `SELECT COALESCE(array_agg(id::text), ARRAY[]::text[]), COALESCE(array_agg(scope_id::text), ARRAY[]::text[]) FROM teams WHERE labels->>$1 = $2`, markerKey, markerValue).Scan(&teamIDs, &teamScopeIDs); err != nil {
		return fmt.Errorf("find sample teams: %w", err)
	}
	if err := tx.QueryRow(ctx, `SELECT COALESCE(array_agg(id::text), ARRAY[]::text[]), COALESCE(array_agg(scope_id::text), ARRAY[]::text[]) FROM projects WHERE labels->>$1 = $2`, markerKey, markerValue).Scan(&projectIDs, &projectScopeIDs); err != nil {
		return fmt.Errorf("find sample projects: %w", err)
	}
	allScopeIDs := append(append([]string{}, teamScopeIDs...), projectScopeIDs...)
	if _, err := tx.Exec(ctx, `DELETE FROM role_bindings WHERE (subject_type = 'user' AND subject_id::text = ANY($1::text[])) OR scope_id::text = ANY($2::text[])`, userIDs, allScopeIDs); err != nil {
		return fmt.Errorf("remove sample role bindings: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id::text = ANY($1::text[])`, userIDs); err != nil {
		return fmt.Errorf("remove sample sessions: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM credentials WHERE user_id::text = ANY($1::text[])`, userIDs); err != nil {
		return fmt.Errorf("remove sample credentials: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM users WHERE id::text = ANY($1::text[])`, userIDs); err != nil {
		return fmt.Errorf("remove sample users: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM projects WHERE id::text = ANY($1::text[])`, projectIDs); err != nil {
		return fmt.Errorf("remove sample projects: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM teams WHERE id::text = ANY($1::text[])`, teamIDs); err != nil {
		return fmt.Errorf("remove sample teams: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM scopes WHERE id::text = ANY($1::text[])`, projectScopeIDs); err != nil {
		return fmt.Errorf("remove sample project scopes: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM scopes WHERE id::text = ANY($1::text[])`, teamScopeIDs); err != nil {
		return fmt.Errorf("remove sample team scopes: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit sample cleanup: %w", err)
	}
	fmt.Println("Removed marked OpsKeeper sample data.")
	return nil
}
