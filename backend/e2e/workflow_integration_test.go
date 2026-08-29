//go:build integration

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/diagnosis"
	"opskeeper/backend/discovery"
	"opskeeper/backend/inspection"
	"opskeeper/backend/migrations"
	"opskeeper/backend/organization"
	"opskeeper/backend/resource"
)

func TestOperatorWorkflowFromImportThroughDiagnosisAndInspection(t *testing.T) {
	pool := workflowPool(t)
	ctx := context.Background()
	organizations := organization.NewService(organization.NewStore(pool))
	team, err := organizations.CreateTeam(ctx, organization.CreateTeamInput{Name: "E2E Team", Code: "e2e-team"})
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	project, err := organizations.CreateProject(ctx, organization.CreateProjectInput{TeamID: team.ID, Name: "E2E Project", Code: "e2e-project"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	var actorID string
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, display_name) VALUES ('e2e@example.com', 'E2E Operator') RETURNING id::text`).Scan(&actorID); err != nil {
		t.Fatalf("create operator: %v", err)
	}

	resources := resource.NewService(resource.NewStore(pool))
	cluster, err := resources.Create(ctx, resource.CreateInput{ScopeID: team.Scope.ID, Kind: "Kubernetes", Name: "e2e-cluster", Config: map[string]any{}})
	if err != nil {
		t.Fatalf("Create(Kubernetes) error = %v", err)
	}
	discoveryStore := discovery.NewStore(pool)
	run, err := discoveryStore.CreateRun(ctx, cluster.ID, actorID)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if err := discoveryStore.SetRunning(ctx, run.ID); err != nil {
		t.Fatalf("SetRunning() error = %v", err)
	}
	if err := discoveryStore.ReplaceItems(ctx, run.ID, []discovery.ScannedItem{
		{Kind: "Project", Namespace: "orders", Name: "orders", ExternalUID: "namespace-orders", Labels: map[string]string{}, Payload: map[string]any{"kubernetes_kind": "Namespace"}},
		{Kind: "Application", Namespace: "orders", Name: "orders-api", ExternalUID: "deployment-orders", Labels: map[string]string{"app": "orders"}, Payload: map[string]any{"kubernetes": map[string]any{"workload_kind": "Deployment"}}},
	}); err != nil {
		t.Fatalf("ReplaceItems() error = %v", err)
	}
	if err := discoveryStore.CompleteRun(ctx, run.ID); err != nil {
		t.Fatalf("CompleteRun() error = %v", err)
	}
	items, err := discoveryStore.ListItems(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	var applicationItemID string
	for _, item := range items {
		if item.Kind == "Application" {
			applicationItemID = item.ID
		}
	}
	discoveryService := discovery.NewService(discoveryStore, resources, resources, organizations, nil, nil)
	imported, err := discoveryService.Import(ctx, actorID, run.ID, discovery.ImportInput{
		ItemIDs: []string{applicationItemID},
		ProjectMappings: map[string]discovery.ProjectMapping{
			"orders": {ProjectID: project.ID},
		},
	})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	var application resource.Resource
	for _, item := range imported.Imported {
		if item.ImportedResourceID != nil {
			application, err = resources.Get(ctx, *item.ImportedResourceID)
			if err != nil {
				t.Fatalf("Get(imported application) error = %v", err)
			}
		}
	}
	if application.ID == "" || application.ScopeID != project.Scope.ID {
		t.Fatalf("imported application = %#v", application)
	}

	postgres, err := resources.Create(ctx, resource.CreateInput{ScopeID: project.Scope.ID, Kind: "PostgreSQL", Name: "orders-db", Config: map[string]any{"host": "db.internal", "port": 5432, "database": "orders"}})
	if err != nil {
		t.Fatalf("Create(PostgreSQL) error = %v", err)
	}
	if _, err := resources.CreateRelation(ctx, actorID, resource.CreateRelationInput{SourceResourceID: application.ID, TargetResourceID: postgres.ID, RelationType: "depends_on"}); err != nil {
		t.Fatalf("CreateRelation() error = %v", err)
	}
	topology, err := resources.Topology(ctx, application.ID, 3, 20)
	if err != nil || len(topology) != 2 {
		t.Fatalf("Topology() = %#v, %v", topology, err)
	}

	diagnosisStore := diagnosis.NewStore(pool)
	diagnosisService := diagnosis.NewService(diagnosisStore, resources)
	session, err := diagnosisService.Start(ctx, diagnosis.StartInput{ScopeID: project.Scope.ID, ActorUserID: actorID, Question: "Why is orders unhealthy?", TargetResourceIDs: []string{application.ID, postgres.ID}})
	if err != nil {
		t.Fatalf("Start(diagnosis) error = %v", err)
	}
	if _, claimed, err := diagnosisStore.ClaimRun(ctx, session.ID); err != nil || !claimed {
		t.Fatalf("ClaimRun() claimed=%v error=%v", claimed, err)
	}
	evidence, err := diagnosisStore.SaveEvidence(ctx, session.ID, diagnosis.CreateEvidenceInput{
		TargetResourceID: postgres.ID, SourceResourceID: postgres.ID, Capability: "postgresql_inspect", CollectedAt: time.Now().UTC(),
		Summary: json.RawMessage(`{"finding_count":1}`), Content: json.RawMessage(`{"waiting_locks":2}`), Untrusted: true,
	})
	if err != nil {
		t.Fatalf("SaveEvidence() error = %v", err)
	}
	if _, err := diagnosisStore.SaveReport(ctx, diagnosis.Report{SessionID: session.ID, Status: "warning", Conclusion: "Waiting locks require verification", Recommendations: json.RawMessage(`[]`), EvidenceIDs: []string{evidence.ID}}); err != nil {
		t.Fatalf("SaveReport() error = %v", err)
	}
	snapshot, err := diagnosisService.Get(ctx, session.ID)
	if err != nil || snapshot.Report == nil || len(snapshot.Evidence) != 1 {
		t.Fatalf("Get(diagnosis) = %#v, %v", snapshot, err)
	}

	inspectionStore := inspection.NewStore(pool)
	inspectionService := inspection.NewService(inspectionStore, resources)
	policy, err := inspectionService.CreatePolicy(ctx, inspection.Policy{
		ScopeID: project.Scope.ID, Name: "orders-db-health", Cron: "*/5 * * * *", Timezone: "UTC",
		TargetResourceIDs: []string{postgres.ID}, TargetLabels: map[string]string{}, Timeout: 30 * time.Second,
		Retries: 1, MaxConcurrent: 1, MaxToolCalls: 4, MaxTokens: 2000, Maintenance: []inspection.MaintenanceWindow{},
	}, actorID)
	if err != nil {
		t.Fatalf("CreatePolicy() error = %v", err)
	}
	if _, err := inspectionService.StartManualRun(ctx, project.Scope.ID, policy.ID, time.Now().UTC()); err != nil {
		t.Fatalf("StartManualRun() error = %v", err)
	}
	worker := inspection.NewWorker(inspectionStore, deterministicChecker{targetID: postgres.ID}, nil, "e2e-worker", 10*time.Second)
	if claimed, err := worker.RunOnce(ctx); err != nil || !claimed {
		t.Fatalf("RunOnce() claimed=%v error=%v", claimed, err)
	}
	findings, err := inspectionService.ListFindings(ctx, project.Scope.ID, 10)
	if err != nil || len(findings) != 1 || findings[0].TargetResourceID != postgres.ID {
		t.Fatalf("ListFindings() = %#v, %v", findings, err)
	}
}

type deterministicChecker struct{ targetID string }

func (c deterministicChecker) Check(context.Context, string) ([]inspection.RuleResult, error) {
	return []inspection.RuleResult{{TargetResourceID: c.targetID, Rule: "postgresql.waiting_locks", Severity: "warning", Weight: 20, Message: "waiting locks detected"}}, nil
}

func workflowPool(t *testing.T) *pgxpool.Pool {
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
	schema := fmt.Sprintf("workflow_test_%d", time.Now().UnixNano())
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
