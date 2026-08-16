//go:build integration

package discovery_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/authorization"
	"opskeeper/backend/discovery"
	"opskeeper/backend/migrations"
	"opskeeper/backend/organization"
	"opskeeper/backend/resource"
)

func TestDiscoveryRunItemsAndResourceVisibility(t *testing.T) {
	pool := discoveryIntegrationPool(t)
	ctx := context.Background()
	organizations := organization.NewService(organization.NewStore(pool))
	team, err := organizations.CreateTeam(ctx, organization.CreateTeamInput{Name: "Discovery Team", Code: "discovery-team"})
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	project, err := organizations.CreateProject(ctx, organization.CreateProjectInput{TeamID: team.ID, Name: "Discovery Project", Code: "discovery-project"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	resources := resource.NewService(resource.NewStore(pool))
	cluster, err := resources.Create(ctx, resource.CreateInput{ScopeID: team.Scope.ID, Kind: "Kubernetes", Name: "cluster", Config: map[string]any{}})
	if err != nil {
		t.Fatalf("Create(Kubernetes) error = %v", err)
	}
	application, err := resources.Import(ctx, resource.ImportedInput{
		ScopeID: project.Scope.ID, Kind: "Application", Name: "orders",
		ExternalUID: "workload-uid", SourceResourceID: cluster.ID, Config: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Import(Application) error = %v", err)
	}
	var actorID string
	if err := pool.QueryRow(ctx, "INSERT INTO users (email, display_name) VALUES ('discovery@example.com', 'Discovery') RETURNING id::text").Scan(&actorID); err != nil {
		t.Fatalf("insert discovery actor: %v", err)
	}

	store := discovery.NewStore(pool)
	run, err := store.CreateRun(ctx, cluster.ID, actorID)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if err := store.SetRunning(ctx, run.ID); err != nil {
		t.Fatalf("SetRunning() error = %v", err)
	}
	if _, err := store.CreateRun(ctx, cluster.ID, actorID); err != discovery.ErrConflict {
		t.Fatalf("CreateRun(concurrent) error = %v, want ErrConflict", err)
	}
	if err := store.ReplaceItems(ctx, run.ID, []discovery.ScannedItem{
		{Kind: "Project", Namespace: "payments", Name: "payments", ExternalUID: "namespace-uid", Labels: map[string]string{}, Payload: map[string]any{"kubernetes_kind": "Namespace"}},
		{Kind: "Application", Namespace: "payments", Name: "orders", ExternalUID: "workload-uid", Labels: map[string]string{"app": "orders"}, Payload: map[string]any{"instances": []any{}}},
	}); err != nil {
		t.Fatalf("ReplaceItems() error = %v", err)
	}
	if err := store.CompleteRun(ctx, run.ID); err != nil {
		t.Fatalf("CompleteRun() error = %v", err)
	}
	items, err := store.ListItems(ctx, run.ID)
	if err != nil || len(items) != 2 {
		t.Fatalf("ListItems() = %#v, %v", items, err)
	}
	for _, item := range items {
		switch item.Kind {
		case "Project":
			if err := store.MarkProjectMapped(ctx, item.ID, project.ID, run.ID); err != nil {
				t.Fatalf("MarkProjectMapped() error = %v", err)
			}
		case "Application":
			if err := store.MarkImported(ctx, item.ID, application.ID, run.ID); err != nil {
				t.Fatalf("MarkImported() error = %v", err)
			}
		}
	}
	completed, err := store.GetRun(ctx, run.ID)
	if err != nil || completed.Status != discovery.RunSucceeded || completed.ItemCount != 2 || completed.ImportedCount != 2 {
		t.Fatalf("completed discovery = %#v, %v", completed, err)
	}

	allowedContext := authorization.WithResourceFilter(ctx, authorization.ResourceFilter{ResourceIDs: []string{cluster.ID}})
	if _, err := store.GetRun(allowedContext, run.ID); err != nil {
		t.Fatalf("GetRun(explicit cluster grant) error = %v", err)
	}
	deniedContext := authorization.WithResourceFilter(ctx, authorization.ResourceFilter{})
	if _, err := store.GetRun(deniedContext, run.ID); err != discovery.ErrNotFound {
		t.Fatalf("GetRun(no cluster grant) error = %v, want ErrNotFound", err)
	}
}

func TestDiscoveryImportHonorsApplicationSelection(t *testing.T) {
	pool := discoveryIntegrationPool(t)
	ctx := context.Background()
	organizations := organization.NewService(organization.NewStore(pool))
	team, err := organizations.CreateTeam(ctx, organization.CreateTeamInput{Name: "Selection Team", Code: "selection-team"})
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	project, err := organizations.CreateProject(ctx, organization.CreateProjectInput{TeamID: team.ID, Name: "Selection Project", Code: "selection-project"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	resources := resource.NewService(resource.NewStore(pool))
	cluster, err := resources.Create(ctx, resource.CreateInput{ScopeID: team.Scope.ID, Kind: "Kubernetes", Name: "selection-cluster", Config: map[string]any{}})
	if err != nil {
		t.Fatalf("Create(Kubernetes) error = %v", err)
	}
	var actorID string
	if err := pool.QueryRow(ctx, "INSERT INTO users (email, display_name) VALUES ('selection@example.com', 'Selection') RETURNING id::text").Scan(&actorID); err != nil {
		t.Fatalf("insert selection actor: %v", err)
	}
	store := discovery.NewStore(pool)
	run, err := store.CreateRun(ctx, cluster.ID, actorID)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if err := store.SetRunning(ctx, run.ID); err != nil {
		t.Fatalf("SetRunning() error = %v", err)
	}
	if err := store.ReplaceItems(ctx, run.ID, []discovery.ScannedItem{
		{Kind: "Project", Namespace: "orders", Name: "orders", ExternalUID: "namespace-selection-uid", Labels: map[string]string{}, Payload: map[string]any{"kubernetes_kind": "Namespace"}},
		{Kind: "Application", Namespace: "orders", Name: "orders-api", ExternalUID: "workload-selection-uid", Labels: map[string]string{}, Payload: map[string]any{"kubernetes": map[string]any{"workload_kind": "Deployment"}}},
	}); err != nil {
		t.Fatalf("ReplaceItems() error = %v", err)
	}
	if err := store.CompleteRun(ctx, run.ID); err != nil {
		t.Fatalf("CompleteRun() error = %v", err)
	}

	service := discovery.NewService(store, resources, resources, organizations, nil, nil)
	result, err := service.Import(ctx, actorID, run.ID, discovery.ImportInput{
		ItemIDs: []string{},
		ProjectMappings: map[string]discovery.ProjectMapping{
			"orders": {ProjectID: project.ID},
		},
	})
	if err != nil {
		t.Fatalf("Import(empty application selection) error = %v", err)
	}
	if len(result.Imported) != 1 || result.Imported[0].Kind != "Project" {
		t.Fatalf("Import(empty application selection) = %#v", result.Imported)
	}
	var applicationCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM resources WHERE source_resource_id = $1 AND kind = 'Application' AND deleted_at IS NULL`, cluster.ID).Scan(&applicationCount); err != nil {
		t.Fatalf("count unselected Applications: %v", err)
	}
	if applicationCount != 0 {
		t.Fatalf("unselected Application count = %d, want 0", applicationCount)
	}

	items, err := store.ListItems(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	var applicationItemID string
	for _, item := range items {
		if item.Kind == "Application" {
			applicationItemID = item.ID
		}
	}
	if applicationItemID == "" {
		t.Fatal("Application discovery item was not found")
	}
	result, err = service.Import(ctx, actorID, run.ID, discovery.ImportInput{
		ItemIDs: []string{applicationItemID},
		ProjectMappings: map[string]discovery.ProjectMapping{
			"orders": {ProjectID: project.ID},
		},
	})
	if err != nil {
		t.Fatalf("Import(selected Application) error = %v", err)
	}
	if len(result.Imported) != 2 {
		t.Fatalf("Import(selected Application) = %#v", result.Imported)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM resources WHERE source_resource_id = $1 AND kind = 'Application' AND deleted_at IS NULL`, cluster.ID).Scan(&applicationCount); err != nil {
		t.Fatalf("count selected Applications: %v", err)
	}
	if applicationCount != 1 {
		t.Fatalf("selected Application count = %d, want 1", applicationCount)
	}
}

func discoveryIntegrationPool(t *testing.T) *pgxpool.Pool {
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
	schema := fmt.Sprintf("discovery_test_%d", time.Now().UnixNano())
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
