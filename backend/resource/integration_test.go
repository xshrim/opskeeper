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
	"opskeeper/backend/discovery"
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
	teamResource, err := service.Create(ctx, resource.CreateInput{ScopeID: teamA.Scope.ID, Kind: "Redis", Name: "shared", Labels: map[string]string{"tier": "shared"}, Config: map[string]any{"host": "redis", "port": 6379}})
	if err != nil {
		t.Fatalf("Create(team resource) error = %v", err)
	}
	projectResource, err := service.Create(ctx, resource.CreateInput{ScopeID: projectA.Scope.ID, Kind: "Application", Name: "orders", Config: map[string]any{}})
	if err != nil {
		t.Fatalf("Create(project resource) error = %v", err)
	}
	otherProjectResource, err := service.Create(ctx, resource.CreateInput{ScopeID: projectB.Scope.ID, Kind: "Application", Name: "orders-http", Config: map[string]any{}})
	if err != nil {
		t.Fatalf("Create(second project resource) error = %v", err)
	}
	cycleResource, err := service.Create(ctx, resource.CreateInput{ScopeID: projectA.Scope.ID, Kind: "Application", Name: "orders-cycle", Config: map[string]any{}})
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

func TestProjectMemberOnlySeesExplicitlyGrantedResources(t *testing.T) {
	pool := resourceIntegrationPool(t)
	ctx := context.Background()
	organizations := organization.NewService(organization.NewStore(pool))
	platform, err := organizations.GetPlatform(ctx)
	if err != nil {
		t.Fatalf("GetPlatform() error = %v", err)
	}
	team, err := organizations.CreateTeam(ctx, organization.CreateTeamInput{Name: "Restricted Team", Code: "restricted-team"})
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	project, err := organizations.CreateProject(ctx, organization.CreateProjectInput{TeamID: team.ID, Name: "Restricted Project", Code: "restricted-project"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	resources := resource.NewService(resource.NewStore(pool))
	allowed, err := resources.Create(ctx, resource.CreateInput{ScopeID: project.Scope.ID, Kind: "Application", Name: "allowed-application", Config: map[string]any{}})
	if err != nil {
		t.Fatalf("Create(allowed resource) error = %v", err)
	}
	denied, err := resources.Create(ctx, resource.CreateInput{ScopeID: project.Scope.ID, Kind: "Application", Name: "denied-application", Config: map[string]any{}})
	if err != nil {
		t.Fatalf("Create(denied resource) error = %v", err)
	}

	userID := insertResourceUser(t, pool, "resource-restricted@example.com")
	if _, err := pool.Exec(ctx, `
		INSERT INTO role_bindings (subject_type, subject_id, role_id, scope_id)
		SELECT 'user', $1::uuid, id, $2::uuid FROM roles WHERE name = 'ProjectMember'`, userID, project.Scope.ID); err != nil {
		t.Fatalf("bind ProjectMember: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO resource_role_bindings (subject_type, subject_id, role_id, resource_id)
		SELECT 'user', $1::uuid, id, $2::uuid FROM resource_roles WHERE name = 'ResourceViewer'`, userID, allowed.ID); err != nil {
		t.Fatalf("bind ResourceViewer: %v", err)
	}

	authorizationService := authorization.NewService(authorization.NewStore(pool))
	organizationFilter, err := authorizationService.ScopeFilter(ctx, authorization.Subject{UserID: userID}, authorization.OrganizationRead)
	if err != nil {
		t.Fatalf("ScopeFilter(organization:read) error = %v", err)
	}
	if !organizationFilter.Allows(platform.Scope.ID) || !organizationFilter.Allows(team.Scope.ID) || !organizationFilter.Allows(project.Scope.ID) {
		t.Fatalf("organization filter = %#v, want project and its navigation ancestors", organizationFilter.ScopeIDs)
	}
	resourceFilter, err := authorizationService.ResourceFilter(ctx, authorization.Subject{UserID: userID}, authorization.ResourceRead)
	if err != nil {
		t.Fatalf("ResourceFilter(resource:read) error = %v", err)
	}
	if len(resourceFilter.ScopeIDs) != 0 || !resourceFilter.Allows(project.Scope.ID, allowed.ID) || resourceFilter.Allows(project.Scope.ID, denied.ID) {
		t.Fatalf("resource filter = %#v", resourceFilter)
	}

	filteredContext := authorization.WithResourceFilter(ctx, resourceFilter)
	page, err := resources.List(filteredContext, resource.Pagination{}, "", nil)
	if err != nil {
		t.Fatalf("List(resource filtered) error = %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != allowed.ID {
		t.Fatalf("resource filtered page = %#v", page)
	}
	if _, err := resources.Get(filteredContext, denied.ID); !errors.Is(err, resource.ErrNotFound) {
		t.Fatalf("Get(denied resource) error = %v, want ErrNotFound", err)
	}
}

func TestImportedApplicationsAreIdempotentAndMissingBecomesUnknown(t *testing.T) {
	pool := resourceIntegrationPool(t)
	ctx := context.Background()
	organizations := organization.NewService(organization.NewStore(pool))
	team, err := organizations.CreateTeam(ctx, organization.CreateTeamInput{Name: "Import Team", Code: "import-team"})
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	project, err := organizations.CreateProject(ctx, organization.CreateProjectInput{TeamID: team.ID, Name: "Import Project", Code: "import-project"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	resources := resource.NewService(resource.NewStore(pool))
	cluster, err := resources.Create(ctx, resource.CreateInput{ScopeID: team.Scope.ID, Kind: "Kubernetes", Name: "cluster-a", Config: map[string]any{}})
	if err != nil {
		t.Fatalf("Create(Kubernetes) error = %v", err)
	}

	first, err := resources.Import(ctx, resource.ImportedInput{
		ScopeID:          project.Scope.ID,
		Kind:             "Application",
		Name:             "orders-v1",
		ExternalUID:      "workload-uid-a",
		SourceResourceID: cluster.ID,
		Config:           map[string]any{"kubernetes": map[string]any{"workload_kind": "Deployment"}},
	})
	if err != nil {
		t.Fatalf("Import(first Application) error = %v", err)
	}
	updated, err := resources.Import(ctx, resource.ImportedInput{
		ScopeID:          project.Scope.ID,
		Kind:             "Application",
		Name:             "orders-v2",
		ExternalUID:      "workload-uid-a",
		SourceResourceID: cluster.ID,
		Config:           map[string]any{"kubernetes": map[string]any{"workload_kind": "Deployment"}, "instances": []any{}},
	})
	if err != nil {
		t.Fatalf("Import(updated Application) error = %v", err)
	}
	missing, err := resources.Import(ctx, resource.ImportedInput{
		ScopeID:          project.Scope.ID,
		Kind:             "Application",
		Name:             "worker",
		ExternalUID:      "workload-uid-b",
		SourceResourceID: cluster.ID,
		Config:           map[string]any{"kubernetes": map[string]any{"workload_kind": "StatefulSet"}},
	})
	if err != nil {
		t.Fatalf("Import(second Application) error = %v", err)
	}
	if updated.ID != first.ID || updated.Name != "orders-v2" {
		t.Fatalf("idempotent import = first %#v, updated %#v", first, updated)
	}

	if err := discovery.NewStore(pool).MarkMissing(ctx, cluster.ID, []string{"workload-uid-a"}); err != nil {
		t.Fatalf("MarkMissing() error = %v", err)
	}
	active, err := resources.Get(ctx, first.ID)
	if err != nil || active.Status != resource.StatusActive {
		t.Fatalf("current Application = %#v, %v", active, err)
	}
	unknown, err := resources.Get(ctx, missing.ID)
	if err != nil || unknown.Status != resource.StatusUnknown {
		t.Fatalf("missing Application = %#v, %v", unknown, err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM resources WHERE scope_id = $1::uuid AND kind = 'Application' AND external_uid = 'workload-uid-a' AND source_resource_id = $2 AND deleted_at IS NULL`, project.Scope.ID, cluster.ID).Scan(&count); err != nil {
		t.Fatalf("count imported Application: %v", err)
	}
	if count != 1 {
		t.Fatalf("imported Application count = %d, want 1", count)
	}
}

func insertResourceUser(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	var userID string
	if err := pool.QueryRow(context.Background(), "INSERT INTO users (email, display_name) VALUES ($1, $1) RETURNING id::text", email).Scan(&userID); err != nil {
		t.Fatalf("insert resource user: %v", err)
	}
	return userID
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
