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

func TestBaselineMigrationRollsBackAndReapplies(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	if err := Apply(ctx, pool); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	var applied int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("count applied migrations: %v", err)
	}
	if applied != 1 {
		t.Fatalf("applied migrations = %d, want 1", applied)
	}
	var tables int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		 WHERE table_schema = current_schema()
		   AND table_name IN ('resources', 'skill_versions', 'diagnosis_sessions')`).Scan(&tables); err != nil {
		t.Fatalf("count baseline tables: %v", err)
	}
	if tables != 3 {
		t.Fatalf("baseline table count = %d, want 3", tables)
	}
	if err := RollbackLast(ctx, pool); err != nil {
		t.Fatalf("RollbackLast() error = %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("count migrations after rollback: %v", err)
	}
	if applied != 0 {
		t.Fatalf("migrations after rollback = %d, want 0", applied)
	}
	var scopesTable *string
	if err := pool.QueryRow(ctx, "SELECT to_regclass(current_schema() || '.scopes')::text").Scan(&scopesTable); err != nil {
		t.Fatalf("check scopes table after rollback: %v", err)
	}
	if scopesTable != nil {
		t.Fatalf("scopes table still exists after rollback: %s", *scopesTable)
	}
	if err := Apply(ctx, pool); err != nil {
		t.Fatalf("reapply baseline error = %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("count migrations after reapply: %v", err)
	}
	if applied != 1 {
		t.Fatalf("migrations after reapply = %d, want 1", applied)
	}
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
