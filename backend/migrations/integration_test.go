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
	var grantedPermissions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM role_permissions permission JOIN roles role ON role.id=permission.role_id WHERE role.name='PlatformAdmin' AND permission.permission=ANY(ARRAY['notification:read','notification:manage','notification:test','notification:retry'])`).Scan(&grantedPermissions); err != nil || grantedPermissions != 4 {
		t.Fatalf("PlatformAdmin notification permissions = %d, %v", grantedPermissions, err)
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
		   AND table_name IN ('users', 'sessions')`).Scan(&identityTables); err != nil {
		t.Fatalf("count identity tables: %v", err)
	}
	if identityTables != 2 {
		t.Fatalf("identity tables = %d, want 2", identityTables)
	}
	var resourceCredentialTables int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*)
		  FROM information_schema.tables
		 WHERE table_schema = current_schema()
		   AND table_name = 'resource_credentials'`).Scan(&resourceCredentialTables); err != nil {
		t.Fatalf("count resource credential tables: %v", err)
	}
	if resourceCredentialTables != 0 {
		t.Fatalf("resource credential tables = %d, want 0", resourceCredentialTables)
	}
	var resourceCredentialColumns int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*)
		  FROM information_schema.columns
		 WHERE table_schema = current_schema()
		   AND table_name = 'resources'
		   AND column_name IN ('credential_id', 'credential_ciphertext', 'credential_key_version', 'credential_purpose')`).Scan(&resourceCredentialColumns); err != nil {
		t.Fatalf("count resource credential columns: %v", err)
	}
	if resourceCredentialColumns != 3 {
		t.Fatalf("resource credential columns = %d, want 3 embedded columns", resourceCredentialColumns)
	}
	var userCredentialTables int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*)
		  FROM information_schema.tables
		 WHERE table_schema = current_schema()
		   AND table_name = 'credentials'`).Scan(&userCredentialTables); err != nil {
		t.Fatalf("count user credential tables: %v", err)
	}
	if userCredentialTables != 0 {
		t.Fatalf("user credential tables = %d, want 0", userCredentialTables)
	}
	var userCredentialColumns int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*)
		  FROM information_schema.columns
		 WHERE table_schema = current_schema()
		   AND table_name = 'users'
		   AND column_name IN ('password_hash', 'password_changed_at', 'must_change_password')`).Scan(&userCredentialColumns); err != nil {
		t.Fatalf("count embedded user credential columns: %v", err)
	}
	if userCredentialColumns != 3 {
		t.Fatalf("embedded user credential columns = %d, want 3", userCredentialColumns)
	}
	var resourceStatusColumns int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'resources' AND column_name = 'status'`).Scan(&resourceStatusColumns); err != nil {
		t.Fatalf("count resource status columns: %v", err)
	}
	if resourceStatusColumns != 0 {
		t.Fatalf("resource status columns = %d, want 0", resourceStatusColumns)
	}
	var notificationCredentialColumns int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*)
		  FROM information_schema.columns
		 WHERE table_schema = current_schema()
		   AND table_name = 'notification_channels'
		   AND column_name = 'credential_id'`).Scan(&notificationCredentialColumns); err != nil {
		t.Fatalf("count notification credential columns: %v", err)
	}
	if notificationCredentialColumns != 0 {
		t.Fatalf("notification credential columns = %d, want 0", notificationCredentialColumns)
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

func TestPublishedNotificationTemplateVersionIsImmutable(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	if err := Apply(ctx, pool); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	var scopeID, templateID, versionID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM scopes WHERE scope_type='platform' ORDER BY created_at LIMIT 1`).Scan(&scopeID); err != nil {
		t.Fatalf("find platform Scope: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO notification_templates(scope_id,name) VALUES($1::uuid,'immutable-template-test') RETURNING id::text`, scopeID).Scan(&templateID); err != nil {
		t.Fatalf("create notification template: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO notification_template_versions(scope_id,template_id,version,format,body_template,variables,content_hash,status,published_at) VALUES($1::uuid,$2::uuid,1,'text','original','[]'::jsonb,repeat('a',64),'published',now()) RETURNING id::text`, scopeID, templateID).Scan(&versionID); err != nil {
		t.Fatalf("create published template version: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE notification_template_versions SET body_template='modified' WHERE id=$1::uuid`, versionID); err == nil {
		t.Fatal("published template content update succeeded")
	}
	if _, err := pool.Exec(ctx, `UPDATE notification_template_versions SET status='disabled' WHERE id=$1::uuid`, versionID); err != nil {
		t.Fatalf("disable published template version: %v", err)
	}
}

func TestBaselineContainsProjectViewerRole(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	if err := Apply(ctx, pool); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM roles WHERE name = 'ProjectViewer' AND scope_type = 'project'`).Scan(&count); err != nil {
		t.Fatalf("count ProjectViewer roles: %v", err)
	}
	if count != 1 {
		t.Fatalf("ProjectViewer roles = %d, want 1", count)
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
