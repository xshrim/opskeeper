//go:build integration

package connector_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/connector"
	"opskeeper/backend/migrations"
	"opskeeper/backend/resource"
)

func TestConnectionCheckStorePersistsLatestResult(t *testing.T) {
	pool := connectorIntegrationPool(t)
	ctx := context.Background()
	var scopeID string
	if err := pool.QueryRow(ctx, "SELECT scope_id::text FROM platforms LIMIT 1").Scan(&scopeID); err != nil {
		t.Fatalf("read platform scope: %v", err)
	}
	item, err := resource.NewService(resource.NewStore(pool)).Create(ctx, resource.CreateInput{
		ScopeID: scopeID, Kind: "Prometheus", Name: "integration-prometheus", Config: map[string]any{"url": "https://prometheus.example"},
	})
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}
	var userID string
	if err := pool.QueryRow(ctx, "INSERT INTO users (email, display_name) VALUES ('connector@example.com', 'Connector') RETURNING id::text").Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	store := connector.NewStore(pool)
	first, err := store.Save(ctx, connector.Check{
		ResourceID: item.ID, Status: "failed", ErrorCategory: connector.CategoryTimeout,
		Message: "连接上游超时", LatencyMS: 1000, CheckedBy: &userID, CheckedAt: time.Now().Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("Save(first) error = %v", err)
	}
	second, err := store.Save(ctx, connector.Check{
		ResourceID: item.ID, Status: "succeeded", Message: "连接测试通过", LatencyMS: 25,
		Capabilities: []connector.Capability{connector.CapabilityQueryMetrics}, CheckedBy: &userID, CheckedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("Save(second) error = %v", err)
	}
	latest, err := store.Latest(ctx, item.ID)
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if first.ID == second.ID || latest.ID != second.ID || latest.Status != "succeeded" || len(latest.Capabilities) != 1 {
		t.Fatalf("checks = first %#v, second %#v, latest %#v", first, second, latest)
	}
}

func connectorIntegrationPool(t *testing.T) *pgxpool.Pool {
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
	schema := fmt.Sprintf("connector_test_%d", time.Now().UnixNano())
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
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		adminPool.Close()
		t.Fatalf("connect integration schema: %v", err)
	}
	if err := migrations.Apply(ctx, pool); err != nil {
		pool.Close()
		_, _ = adminPool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		adminPool.Close()
		t.Fatalf("apply migrations: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = adminPool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		adminPool.Close()
	})
	return pool
}
