//go:build integration

package migrations

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestApplyWaitsForAdvisoryLock(t *testing.T) {
	pool := integrationPool(t)
	blocker, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire blocking connection: %v", err)
	}
	locked := true
	if _, err := blocker.Exec(context.Background(), "SELECT pg_advisory_lock($1)", migrationAdvisoryLockID); err != nil {
		blocker.Release()
		t.Fatalf("acquire blocking advisory lock: %v", err)
	}
	t.Cleanup(func() {
		if locked {
			_, _ = blocker.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationAdvisoryLockID)
		}
		blocker.Release()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- Apply(ctx, pool)
	}()

	select {
	case err := <-result:
		t.Fatalf("Apply() returned before advisory lock was released: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	var unlocked bool
	if err := blocker.QueryRow(context.Background(),
		"SELECT pg_advisory_unlock($1)",
		migrationAdvisoryLockID,
	).Scan(&unlocked); err != nil {
		t.Fatalf("release blocking advisory lock: %v", err)
	}
	if !unlocked {
		t.Fatal("blocking advisory lock was not held")
	}
	locked = false
	blocker.Release()

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("Apply() did not complete after advisory lock release: %v", ctx.Err())
	}
}

func TestConcurrentApplyIsIdempotent(t *testing.T) {
	pool := integrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := make(chan struct{})
	errorsByRun := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			errorsByRun <- Apply(ctx, pool)
		}()
	}
	close(start)
	wait.Wait()
	close(errorsByRun)

	for err := range errorsByRun {
		if err != nil {
			t.Fatalf("concurrent Apply() error = %v", err)
		}
	}

	items, err := load()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	var applied int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM schema_migrations").Scan(&applied); err != nil {
		t.Fatalf("count applied migrations: %v", err)
	}
	if applied != len(items) {
		t.Fatalf("applied migrations = %d, want %d", applied, len(items))
	}
	var checksummed int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM schema_migrations WHERE checksum IS NOT NULL").Scan(&checksummed); err != nil {
		t.Fatalf("count checksummed migrations: %v", err)
	}
	if checksummed != len(items) {
		t.Fatalf("checksummed migrations = %d, want %d", checksummed, len(items))
	}
	var platforms int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM platforms").Scan(&platforms); err != nil {
		t.Fatalf("count platforms: %v", err)
	}
	if platforms != 1 {
		t.Fatalf("platform count = %d, want 1", platforms)
	}
	var organizationStatusColumns int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*)
		  FROM information_schema.columns
		 WHERE table_schema = current_schema()
		   AND table_name IN ('platforms', 'teams', 'projects')
		   AND column_name = 'status'`).Scan(&organizationStatusColumns); err != nil {
		t.Fatalf("count organization status columns: %v", err)
	}
	if organizationStatusColumns != 0 {
		t.Fatalf("organization status columns = %d, want 0", organizationStatusColumns)
	}
	var identityTables int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*)
		  FROM information_schema.tables
		 WHERE table_schema = current_schema()
		   AND table_name IN ('users', 'credentials', 'sessions')`).Scan(&identityTables); err != nil {
		t.Fatalf("count identity tables: %v", err)
	}
	if identityTables != 3 {
		t.Fatalf("identity tables = %d, want 3", identityTables)
	}
}

func TestAuditRetentionIsAppendOnlyAndRecorded(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	if err := Apply(ctx, pool); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	var eventID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO audit_events (action, result, details, created_at)
		VALUES ('retention.test', 'success', '{}'::jsonb, now() - interval '90 days')
		RETURNING id::text`).Scan(&eventID); err != nil {
		t.Fatalf("insert audit event: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE audit_events SET action = 'tampered' WHERE id = $1::uuid`, eventID); err == nil {
		t.Fatal("UPDATE audit_events succeeded, want append-only rejection")
	}
	var removed int64
	if err := pool.QueryRow(ctx, `SELECT prune_audit_events(now() - interval '60 days', 's3://audit/export-001', 'integration-test')`).Scan(&removed); err != nil {
		t.Fatalf("prune audit events: %v", err)
	}
	if removed != 1 {
		t.Fatalf("prune removed = %d, want 1", removed)
	}
	var runs int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_retention_runs WHERE export_reference = 's3://audit/export-001' AND deleted_count = 1`).Scan(&runs); err != nil {
		t.Fatalf("count retention runs: %v", err)
	}
	if runs != 1 {
		t.Fatalf("retention runs = %d, want 1", runs)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM audit_retention_runs`); err == nil {
		t.Fatal("DELETE audit_retention_runs succeeded, want append-only rejection")
	}
}

func TestApplyRejectsChecksumMismatch(t *testing.T) {
	pool := integrationPool(t)
	if err := Apply(context.Background(), pool); err != nil {
		t.Fatalf("initial Apply() error = %v", err)
	}
	if _, err := pool.Exec(context.Background(), `
		UPDATE schema_migrations
		   SET checksum = 'tampered'
		 WHERE version = (SELECT min(version) FROM schema_migrations)`); err != nil {
		t.Fatalf("tamper migration checksum: %v", err)
	}

	err := Apply(context.Background(), pool)
	if err == nil || !strings.Contains(err.Error(), "checksum does not match") {
		t.Fatalf("Apply() error = %v, want checksum mismatch", err)
	}
}

func TestLLMSkillMigrationRollsBackAndReapplies(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	if err := Apply(ctx, pool); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	assertT10Tables := func(want int) {
		t.Helper()
		var count int
		if err := pool.QueryRow(ctx, `
			SELECT count(*) FROM information_schema.tables
			 WHERE table_schema = current_schema()
			   AND table_name IN ('llm_scope_defaults', 'skill_versions', 'skill_scope_defaults', 'skill_executions', 'skill_tool_calls')`).Scan(&count); err != nil {
			t.Fatalf("count T10 tables: %v", err)
		}
		if count != want {
			t.Fatalf("T10 table count = %d, want %d", count, want)
		}
	}
	assertT10Tables(5)
	for range 3 { // 0019 usernames, 0018 audit retention, and 0017 MCP schema.
		if err := RollbackLast(ctx, pool); err != nil {
			t.Fatalf("RollbackLast() error = %v", err)
		}
	}
	for range 3 { // 0016 operations, 0015 inspection, then 0014 contract. 0013 has historical data-only rollback constraints.
		if err := RollbackLast(ctx, pool); err != nil {
			t.Fatalf("RollbackLast() error = %v", err)
		}
	}
	assertT10Tables(5)
	if err := Apply(ctx, pool); err != nil {
		t.Fatalf("second Apply() error = %v", err)
	}
	assertT10Tables(5)
	var schemas int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM resource_schemas WHERE kind IN ('LLMProvider', 'Skill', 'MCPServer') AND version = 2`).Scan(&schemas); err != nil {
		t.Fatalf("count T10 schemas: %v", err)
	}
	if schemas != 3 {
		t.Fatalf("T10 resource schemas = %d, want 3", schemas)
	}
}

func TestDiagnosisMigrationRollsBackAndReapplies(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	if err := Apply(ctx, pool); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	assertT11Tables := func(want int) {
		t.Helper()
		var count int
		if err := pool.QueryRow(ctx, `
			SELECT count(*) FROM information_schema.tables
			 WHERE table_schema = current_schema()
			   AND table_name IN ('diagnosis_sessions', 'diagnosis_targets', 'diagnosis_messages', 'diagnosis_plans', 'diagnosis_plan_steps', 'diagnosis_events', 'diagnosis_evidence', 'diagnosis_hypotheses', 'diagnosis_reports')`).Scan(&count); err != nil {
			t.Fatalf("count T11 tables: %v", err)
		}
		if count != want {
			t.Fatalf("T11 table count = %d, want %d", count, want)
		}
	}
	assertT11Tables(9)
	for range 3 { // 0019 usernames, 0018 audit retention, and 0017 MCP schema.
		if err := RollbackLast(ctx, pool); err != nil {
			t.Fatalf("RollbackLast() error = %v", err)
		}
	}
	for range 3 { // 0016 operations, 0015 inspection, then 0014 contract.
		if err := RollbackLast(ctx, pool); err != nil {
			t.Fatalf("RollbackLast() error = %v", err)
		}
	}
	assertT11Tables(9)
	if err := Apply(ctx, pool); err != nil {
		t.Fatalf("second Apply() error = %v", err)
	}
	assertT11Tables(9)
}

func TestBuiltinSkillsMigrationRollsBackAndReapplies(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	if err := Apply(ctx, pool); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	assertBuiltins := func(want, version int) {
		t.Helper()
		var resources, versions int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM resources WHERE kind = 'Skill' AND config->>'owner' = 'OpsKeeper builtin'`).Scan(&resources); err != nil {
			t.Fatalf("count built-in skill resources: %v", err)
		}
		if err := pool.QueryRow(ctx, `
			SELECT count(*)
			  FROM skill_versions version
			  JOIN resources resource ON resource.id = version.skill_resource_id
			 WHERE resource.kind = 'Skill'
			   AND resource.config->>'owner' = 'OpsKeeper builtin'
			   AND version.version = $1
			   AND version.status = 'published'`, version).Scan(&versions); err != nil {
			t.Fatalf("count built-in skill versions: %v", err)
		}
		if resources != want || versions != want {
			t.Fatalf("built-in resources/versions = %d/%d, want %d/%d", resources, versions, want, want)
		}
	}
	assertBuiltins(4, 2)
	var legacyPublished int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		  FROM skill_versions version
		  JOIN resources resource ON resource.id = version.skill_resource_id
		 WHERE resource.config->>'owner' = 'OpsKeeper builtin'
		   AND version.version = 1
		   AND version.status = 'published'`).Scan(&legacyPublished); err != nil {
		t.Fatalf("count built-in legacy versions: %v", err)
	}
	if legacyPublished != 0 {
		t.Fatalf("published built-in v1 versions = %d, want 0", legacyPublished)
	}
	var middlewareTools int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM skill_versions
		 WHERE version = 2
		   AND (tools @> '[{"name":"connector_postgresql_inspect"}]'::jsonb
		     OR tools @> '[{"name":"connector_redis_inspect"}]'::jsonb
		     OR tools @> '[{"name":"connector_kafka_inspect"}]'::jsonb)`).Scan(&middlewareTools); err != nil {
		t.Fatalf("count built-in middleware tools: %v", err)
	}
	if middlewareTools != 3 {
		t.Fatalf("built-in middleware tools = %d, want 3", middlewareTools)
	}
	var structuredOutputs int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		  FROM skill_versions version
		  JOIN resources resource ON resource.id = version.skill_resource_id
		 WHERE resource.config->>'owner' = 'OpsKeeper builtin'
		   AND version.version = 2
		   AND (version.output_schema->'required') ?& ARRAY['facts', 'findings', 'evidence', 'hypotheses', 'confidence', 'recommendations']`).Scan(&structuredOutputs); err != nil {
		t.Fatalf("count built-in structured outputs: %v", err)
	}
	if structuredOutputs != 4 {
		t.Fatalf("built-in structured outputs = %d, want 4", structuredOutputs)
	}
	if err := RollbackLast(ctx, pool); err != nil { // 0019 usernames
		t.Fatalf("RollbackLast() error = %v", err)
	}
	if err := RollbackLast(ctx, pool); err != nil { // 0018 audit retention
		t.Fatalf("RollbackLast() error = %v", err)
	}
	if err := RollbackLast(ctx, pool); err != nil { // 0017 MCP schema
		t.Fatalf("RollbackLast() error = %v", err)
	}
	assertBuiltins(4, 2)
	if err := RollbackLast(ctx, pool); err != nil { // 0016 operations
		t.Fatalf("RollbackLast() error = %v", err)
	}
	assertBuiltins(4, 2)
	if err := RollbackLast(ctx, pool); err != nil { // 0015 inspection
		t.Fatalf("RollbackLast() error = %v", err)
	}
	assertBuiltins(4, 2)
	if err := RollbackLast(ctx, pool); err != nil { // 0014 contract
		t.Fatalf("RollbackLast() error = %v", err)
	}
	assertBuiltins(4, 1)
	if err := Apply(ctx, pool); err != nil {
		t.Fatalf("second Apply() error = %v", err)
	}
	assertBuiltins(4, 2)
}

func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("OPSK_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("OPSK_TEST_DATABASE_URL is required")
	}

	ctx := context.Background()
	adminPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect integration database: %v", err)
	}
	schema := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		adminPool.Close()
		t.Fatalf("create integration schema: %v", err)
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		adminPool.Close()
		t.Fatalf("parse integration database config: %v", err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	config.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		adminPool.Close()
		t.Fatalf("connect integration schema: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		_, _ = adminPool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		adminPool.Close()
	})
	return pool
}
