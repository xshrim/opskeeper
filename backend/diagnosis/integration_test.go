//go:build integration

package diagnosis_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/diagnosis"
	"opskeeper/backend/migrations"
	"opskeeper/backend/resource"
)

func TestStorePersistsTraceableEvidenceAndRejectsCrossSessionReference(t *testing.T) {
	pool := diagnosisIntegrationPool(t)
	ctx := context.Background()
	var scopeID string
	if err := pool.QueryRow(ctx, "SELECT scope_id::text FROM platforms LIMIT 1").Scan(&scopeID); err != nil {
		t.Fatalf("read platform scope: %v", err)
	}
	resources := resource.NewService(resource.NewStore(pool))
	target, err := resources.Create(ctx, resource.CreateInput{ScopeID: scopeID, Kind: "Application", Name: "diagnosis-target", Config: map[string]any{}})
	if err != nil {
		t.Fatalf("create target resource: %v", err)
	}
	store := diagnosis.NewStore(pool)
	first, err := store.Start(ctx, diagnosis.StartInput{ScopeID: scopeID, Question: "why", TargetResourceIDs: []string{target.ID}})
	if err != nil {
		t.Fatalf("Start(first): %v", err)
	}
	if _, claimed, err := store.ClaimRun(ctx, first.ID); err != nil || !claimed {
		t.Fatalf("ClaimRun() claimed=%v err=%v", claimed, err)
	}
	evidence, err := store.SaveEvidence(ctx, first.ID, diagnosis.CreateEvidenceInput{TargetResourceID: target.ID, SourceResourceID: target.ID, Capability: "query_metrics", CollectedAt: time.Now(), Summary: json.RawMessage(`{"series":1}`), Content: json.RawMessage(`{"error_rate":0.12}`), Untrusted: true})
	if err != nil || len(evidence.ContentHash) != 64 || !evidence.Untrusted {
		t.Fatalf("SaveEvidence() = %#v, %v", evidence, err)
	}
	report, err := store.SaveReport(ctx, diagnosis.Report{SessionID: first.ID, Status: "succeeded", Conclusion: "supported", Recommendations: json.RawMessage(`[]`), EvidenceIDs: []string{evidence.ID}})
	if err != nil || len(report.EvidenceIDs) != 1 {
		t.Fatalf("SaveReport() = %#v, %v", report, err)
	}
	second, err := store.Start(ctx, diagnosis.StartInput{ScopeID: scopeID, Question: "other", TargetResourceIDs: []string{target.ID}})
	if err != nil {
		t.Fatalf("Start(second): %v", err)
	}
	if _, err := store.SaveReport(ctx, diagnosis.Report{SessionID: second.ID, Status: "warning", Conclusion: "unverified", Recommendations: json.RawMessage(`[]`), EvidenceIDs: []string{evidence.ID}}); !diagnosis.IsInvalid(err) {
		t.Fatalf("cross-session evidence reference error = %v, want invalid", err)
	}
}

func diagnosisIntegrationPool(t *testing.T) *pgxpool.Pool {
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
	schema := fmt.Sprintf("diagnosis_test_%d", time.Now().UnixNano())
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
