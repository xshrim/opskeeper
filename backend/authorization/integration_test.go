//go:build integration

package authorization_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/authorization"
	"opskeeper/backend/identity"
	"opskeeper/backend/migrations"
	"opskeeper/backend/organization"
)

func TestRoleInheritanceAndOrganizationFiltering(t *testing.T) {
	pool := authorizationIntegrationPool(t)
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

	teamUser := insertUser(t, pool, "team-viewer@example.com")
	bindRole(t, pool, teamUser, "TeamViewer", teamA.Scope.ID)
	service := authorization.NewService(authorization.NewStore(pool))
	filter, err := service.ScopeFilter(ctx, authorization.Subject{UserID: teamUser}, authorization.OrganizationRead)
	if err != nil {
		t.Fatalf("ScopeFilter(team viewer) error = %v", err)
	}
	if !filter.Allows(platform.Scope.ID) || !filter.Allows(teamA.Scope.ID) || !filter.Allows(projectA.Scope.ID) || filter.Allows(teamB.Scope.ID) || filter.Allows(projectB.Scope.ID) {
		t.Fatalf("team viewer scope filter = %#v", filter.ScopeIDs)
	}
	if err := service.Authorize(ctx, authorization.Subject{UserID: teamUser}, authorization.OrganizationRead, projectA.Scope.ID); err != nil {
		t.Fatalf("Authorize(project A) error = %v", err)
	}
	if err := service.Authorize(ctx, authorization.Subject{UserID: teamUser}, authorization.OrganizationRead, projectB.Scope.ID); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("Authorize(project B) error = %v", err)
	}

	filteredContext := authorization.WithScopeFilter(ctx, filter)
	page, err := organizationService.ListTeams(filteredContext, organization.Pagination{})
	if err != nil {
		t.Fatalf("ListTeams(filtered) error = %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != teamA.ID {
		t.Fatalf("filtered teams = %#v", page)
	}
	if _, err := organizationService.GetTeam(filteredContext, teamB.ID); !errors.Is(err, organization.ErrNotFound) {
		t.Fatalf("GetTeam(out of scope) error = %v", err)
	}
	if _, err := organizationService.GetProject(filteredContext, projectB.ID); !errors.Is(err, organization.ErrNotFound) {
		t.Fatalf("GetProject(out of scope) error = %v", err)
	}

	if _, err := pool.Exec(ctx, "UPDATE scopes SET status = 'disabled' WHERE id = $1::uuid", platform.Scope.ID); err != nil {
		t.Fatalf("disable platform scope: %v", err)
	}
	filter, err = service.ScopeFilter(ctx, authorization.Subject{UserID: teamUser}, authorization.OrganizationRead)
	if err != nil {
		t.Fatalf("ScopeFilter(disabled ancestor) error = %v", err)
	}
	if len(filter.ScopeIDs) != 0 {
		t.Fatalf("disabled ancestor filter = %#v, want empty", filter.ScopeIDs)
	}
	if _, err := pool.Exec(ctx, "UPDATE scopes SET status = 'active' WHERE id = $1::uuid", platform.Scope.ID); err != nil {
		t.Fatalf("restore platform scope: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE scopes SET status = 'disabled' WHERE id = $1::uuid", teamA.Scope.ID); err != nil {
		t.Fatalf("disable bound team scope: %v", err)
	}
	filter, err = service.ScopeFilter(ctx, authorization.Subject{UserID: teamUser}, authorization.OrganizationRead)
	if err != nil {
		t.Fatalf("ScopeFilter(disabled bound scope) error = %v", err)
	}
	if len(filter.ScopeIDs) != 0 {
		t.Fatalf("disabled bound scope filter = %#v, want empty", filter.ScopeIDs)
	}
}

func TestBootstrapAdminReceivesPlatformAdminBinding(t *testing.T) {
	pool := authorizationIntegrationPool(t)
	ctx := context.Background()
	identityService := identity.NewService(identity.NewStore(pool), 15*time.Minute, 7*24*time.Hour)
	admin, err := identityService.BootstrapAdmin(ctx, identity.BootstrapInput{Username: "bootstrap", Email: "bootstrap@example.com", Password: "T04 integration password"})
	if err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	service := authorization.NewService(authorization.NewStore(pool))
	if err := service.EnsureBootstrapAdmin(ctx, admin.ID); err != nil {
		t.Fatalf("EnsureBootstrapAdmin() error = %v", err)
	}
	filter, err := service.ScopeFilter(ctx, authorization.Subject{UserID: admin.ID}, authorization.TeamManage)
	if err != nil {
		t.Fatalf("ScopeFilter(bootstrap admin) error = %v", err)
	}
	if len(filter.ScopeIDs) != 1 {
		t.Fatalf("bootstrap admin team scope filter = %#v", filter.ScopeIDs)
	}
}

func TestRoleSeedAndBindingScopeConstraint(t *testing.T) {
	pool := authorizationIntegrationPool(t)
	ctx := context.Background()
	var roleCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM roles WHERE builtin").Scan(&roleCount); err != nil {
		t.Fatalf("count builtin roles: %v", err)
	}
	if roleCount != 10 {
		t.Fatalf("builtin roles = %d, want 10", roleCount)
	}
	var permissionCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM role_permissions").Scan(&permissionCount); err != nil {
		t.Fatalf("count role permissions: %v", err)
	}
	if permissionCount < 20 {
		t.Fatalf("role permissions = %d, want at least 20", permissionCount)
	}
	var platformScopeID string
	if err := pool.QueryRow(ctx, "SELECT scope_id::text FROM platforms WHERE code = 'default'").Scan(&platformScopeID); err != nil {
		t.Fatalf("find platform scope: %v", err)
	}
	userID := insertUser(t, pool, "scope-constraint@example.com")
	_, err := pool.Exec(ctx, `
		INSERT INTO role_bindings (subject_type, subject_id, role_id, scope_id)
		SELECT 'user', $1::uuid, role.id, $2::uuid FROM roles role WHERE role.name = 'ProjectViewer'`, userID, platformScopeID)
	if err == nil {
		t.Fatal("ProjectViewer binding accepted at platform scope")
	}
}

func insertUser(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	var userID string
	if err := pool.QueryRow(context.Background(), "INSERT INTO users (email, display_name) VALUES ($1, $2) RETURNING id::text", email, email).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return userID
}

func bindRole(t *testing.T, pool *pgxpool.Pool, userID, roleName, scopeID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO role_bindings (subject_type, subject_id, role_id, scope_id)
		SELECT 'user', $1::uuid, role.id, $3::uuid FROM roles role WHERE role.name = $2`, userID, roleName, scopeID); err != nil {
		t.Fatalf("bind role: %v", err)
	}
}

func authorizationIntegrationPool(t *testing.T) *pgxpool.Pool {
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
	schema := fmt.Sprintf("authorization_test_%d", time.Now().UnixNano())
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
	config.MaxConns = 6
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
