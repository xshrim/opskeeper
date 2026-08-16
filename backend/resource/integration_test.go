//go:build integration

package resource_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/authorization"
	"opskeeper/backend/credential"
	"opskeeper/backend/migrations"
	"opskeeper/backend/organization"
	"opskeeper/backend/resource"
)

func TestResourceScopeRelationsDefaultsAndCredentialBoundary(t *testing.T) {
	pool := resourceIntegrationPool(t)
	ctx := context.Background()
	organizationService := organization.NewService(organization.NewStore(pool))
	platform, err := organizationService.GetPlatform(ctx)
	if err != nil {
		t.Fatalf("GetPlatform() error = %v", err)
	}
	teamA, err := organizationService.CreateTeam(ctx, organization.CreateTeamInput{Name: "Team A", Code: "team-a"})
	if err != nil {
		t.Fatalf("CreateTeam(A) error = %v", err)
	}
	teamB, err := organizationService.CreateTeam(ctx, organization.CreateTeamInput{Name: "Team B", Code: "team-b"})
	if err != nil {
		t.Fatalf("CreateTeam(B) error = %v", err)
	}
	projectA, err := organizationService.CreateProject(ctx, organization.CreateProjectInput{TeamID: teamA.ID, Name: "Project A", Code: "project-a"})
	if err != nil {
		t.Fatalf("CreateProject(A) error = %v", err)
	}
	projectB, err := organizationService.CreateProject(ctx, organization.CreateProjectInput{TeamID: teamB.ID, Name: "Project B", Code: "project-b"})
	if err != nil {
		t.Fatalf("CreateProject(B) error = %v", err)
	}

	service := resource.NewService(resource.NewStore(pool))
	platformResource, err := service.Create(ctx, resource.CreateInput{ScopeID: platform.Scope.ID, Kind: "Prometheus", Name: "main", Config: map[string]any{"endpoint": "https://prometheus.example"}})
	if err != nil {
		t.Fatalf("Create(platform resource) error = %v", err)
	}
	teamResource, err := service.Create(ctx, resource.CreateInput{ScopeID: teamA.Scope.ID, Kind: "Redis", Name: "shared", Labels: map[string]string{"tier": "shared"}, Config: map[string]any{"address": "redis:6379"}})
	if err != nil {
		t.Fatalf("Create(team resource) error = %v", err)
	}
	projectResource, err := service.Create(ctx, resource.CreateInput{ScopeID: projectA.Scope.ID, Kind: "BusinessApplication", Name: "orders", Config: map[string]any{}})
	if err != nil {
		t.Fatalf("Create(project resource) error = %v", err)
	}
	otherProjectResource, err := service.Create(ctx, resource.CreateInput{ScopeID: projectB.Scope.ID, Kind: "Endpoint", Name: "orders-http", Config: map[string]any{}})
	if err != nil {
		t.Fatalf("Create(second project resource) error = %v", err)
	}
	cycleResource, err := service.Create(ctx, resource.CreateInput{ScopeID: projectA.Scope.ID, Kind: "Endpoint", Name: "orders-cycle", Config: map[string]any{}})
	if err != nil {
		t.Fatalf("Create(cycle resource) error = %v", err)
	}

	if _, err := service.CreateRelation(ctx, "", resource.CreateRelationInput{SourceResourceID: projectResource.ID, TargetResourceID: teamResource.ID, RelationType: "depends_on"}); err != nil {
		t.Fatalf("CreateRelation(project -> team) error = %v", err)
	}
	if _, err := service.CreateRelation(ctx, "", resource.CreateRelationInput{SourceResourceID: projectResource.ID, TargetResourceID: platformResource.ID, RelationType: "observed_by"}); err != nil {
		t.Fatalf("CreateRelation(project -> platform) error = %v", err)
	}
	if _, err := service.CreateRelation(ctx, "", resource.CreateRelationInput{SourceResourceID: projectResource.ID, TargetResourceID: otherProjectResource.ID, RelationType: "depends_on"}); err == nil {
		t.Fatal("CreateRelation(project -> same-team project) error = nil")
	}
	if _, err := service.CreateRelation(ctx, "", resource.CreateRelationInput{SourceResourceID: cycleResource.ID, TargetResourceID: projectResource.ID, RelationType: "depends_on"}); err != nil {
		t.Fatalf("CreateRelation(same-scope edge) error = %v", err)
	}
	if _, err := service.CreateRelation(ctx, "", resource.CreateRelationInput{SourceResourceID: projectResource.ID, TargetResourceID: cycleResource.ID, RelationType: "observed_by"}); !errors.Is(err, resource.ErrRelationCycle) {
		t.Fatalf("CreateRelation(cycle) error = %v, want ErrRelationCycle", err)
	}

	if _, err := service.SetDefault(ctx, teamA.Scope.ID, "cache", teamResource.ID); err != nil {
		t.Fatalf("SetDefault() error = %v", err)
	}
	resolved, err := service.ResolveDefault(ctx, projectA.Scope.ID, "cache")
	if err != nil || resolved.ID != teamResource.ID {
		t.Fatalf("ResolveDefault() = %#v, %v", resolved, err)
	}
	topology, err := service.Topology(ctx, projectResource.ID, 5, 20)
	if err != nil || len(topology) < 3 {
		t.Fatalf("Topology() = %#v, %v", topology, err)
	}

	if _, err := service.Update(ctx, projectResource.ID, resource.UpdateInput{ScopeID: &projectB.Scope.ID}); err == nil {
		t.Fatal("scope move that invalidates relation was accepted")
	}
	filteredContext := authorization.WithScopeFilter(ctx, authorization.ScopeFilter{ScopeIDs: []string{projectA.Scope.ID}})
	visible, err := service.List(filteredContext, resource.Pagination{}, "", nil)
	if err != nil {
		t.Fatalf("List(filtered) error = %v", err)
	}
	if visible.Total != 4 {
		t.Fatalf("List(filtered) total = %d, want project resources plus team/platform ancestors", visible.Total)
	}

	var userID string
	if err := pool.QueryRow(ctx, "INSERT INTO users (email, display_name) VALUES ('resource-credential@example.com', 'Resource Credential') RETURNING id::text").Scan(&userID); err != nil {
		t.Fatalf("insert credential actor: %v", err)
	}
	encryptor, err := credential.NewLocalEncryptor([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatalf("NewLocalEncryptor() error = %v", err)
	}
	credentialService := credential.NewService(credential.NewStore(pool), encryptor)
	created, err := credentialService.Create(ctx, userID, credential.CreateInput{ScopeID: teamA.Scope.ID, Name: "redis-auth", Secret: "super-secret-value"})
	if err != nil {
		t.Fatalf("Create(credential) error = %v", err)
	}
	if created.Name != "redis-auth" || created.KeyVersion != "local-v1" {
		t.Fatalf("created credential = %#v", created)
	}
	var ciphertext []byte
	if err := pool.QueryRow(ctx, "SELECT ciphertext FROM resource_credentials WHERE id = $1::uuid", created.ID).Scan(&ciphertext); err != nil {
		t.Fatalf("read credential ciphertext: %v", err)
	}
	if strings.Contains(string(ciphertext), "super-secret-value") {
		t.Fatal("credential ciphertext contains plaintext")
	}
}

func resourceIntegrationPool(t *testing.T) *pgxpool.Pool {
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
	schema := fmt.Sprintf("resource_test_%d", time.Now().UnixNano())
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
	config.MaxConns = 8
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
