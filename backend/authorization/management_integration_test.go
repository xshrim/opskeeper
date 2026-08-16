//go:build integration

package authorization_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"opskeeper/backend/audit"
	"opskeeper/backend/authorization"
	"opskeeper/backend/identity"
	"opskeeper/backend/organization"
)

type memoryScopeCache struct {
	mu     sync.Mutex
	values map[string]authorization.ScopeFilter
}

func (c *memoryScopeCache) Get(_ context.Context, key string) (authorization.ScopeFilter, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.values[key]
	return value, ok, nil
}

func (c *memoryScopeCache) Set(_ context.Context, key string, value authorization.ScopeFilter, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.values == nil {
		c.values = make(map[string]authorization.ScopeFilter)
	}
	c.values[key] = value
	return nil
}

func TestGroupRoleBindingAndEscalationBoundary(t *testing.T) {
	pool := authorizationIntegrationPool(t)
	ctx := context.Background()
	org := organization.NewService(organization.NewStore(pool))
	platform, err := org.GetPlatform(ctx)
	if err != nil {
		t.Fatalf("GetPlatform() error = %v", err)
	}
	team, err := org.CreateTeam(ctx, organization.CreateTeamInput{Name: "Managed Team", Code: "managed-team"})
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	project, err := org.CreateProject(ctx, organization.CreateProjectInput{TeamID: team.ID, Name: "Managed Project", Code: "managed-project"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	identityService := identity.NewService(identity.NewStore(pool), 15*time.Minute, 7*24*time.Hour)
	admin, err := identityService.BootstrapAdmin(ctx, identity.BootstrapInput{Email: "t05-platform@example.com", Password: "T05 integration password"})
	if err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	authorizationService := authorization.NewService(authorization.NewStore(pool, &memoryScopeCache{}))
	if err := authorizationService.EnsureBootstrapAdmin(ctx, admin.ID); err != nil {
		t.Fatalf("EnsureBootstrapAdmin() error = %v", err)
	}
	managedUser := insertUser(t, pool, "t05-managed@example.com")
	teamAdmin := insertUser(t, pool, "t05-team-admin@example.com")
	bindRole(t, pool, teamAdmin, "TeamAdmin", team.Scope.ID)

	auditService := audit.NewService(audit.NewStore(pool))
	management := authorization.NewManagementService(authorization.NewManagementStore(pool), authorizationService, auditService)
	group, err := management.CreateGroup(ctx, admin.ID, authorization.CreateGroupInput{ScopeID: team.Scope.ID, Name: "Read only", Description: "T05 group"}, authorizationEvent(admin.ID))
	if err != nil {
		t.Fatalf("CreateGroup() error = %v", err)
	}
	if _, err := management.AddGroupMember(ctx, admin.ID, group.ID, managedUser, authorizationEvent(admin.ID)); err != nil {
		t.Fatalf("AddGroupMember() error = %v", err)
	}
	roles, err := management.ListRoles(ctx, admin.ID)
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	var teamViewerID string
	for _, role := range roles {
		if role.Name == "TeamViewer" {
			teamViewerID = role.ID
		}
	}
	if teamViewerID == "" {
		t.Fatal("TeamViewer role was not returned")
	}
	beforeBinding, err := authorizationService.ScopeFilter(ctx, authorization.Subject{UserID: managedUser}, authorization.OrganizationRead)
	if err != nil {
		t.Fatalf("ScopeFilter(before binding) error = %v", err)
	}
	if len(beforeBinding.ScopeIDs) != 0 {
		t.Fatalf("before binding filter = %#v, want empty", beforeBinding.ScopeIDs)
	}
	if _, err := management.CreateRoleBinding(ctx, admin.ID, authorization.GrantRoleInput{SubjectType: "group", SubjectID: group.ID, RoleID: teamViewerID, ScopeID: team.Scope.ID}, authorizationEvent(admin.ID)); err != nil {
		t.Fatalf("CreateRoleBinding() error = %v", err)
	}
	filter, err := authorizationService.ScopeFilter(ctx, authorization.Subject{UserID: managedUser}, authorization.OrganizationRead)
	if err != nil {
		t.Fatalf("ScopeFilter(group member) error = %v", err)
	}
	if !filter.Allows(team.Scope.ID) || !filter.Allows(project.Scope.ID) || filter.Allows(platform.Scope.ID) {
		t.Fatalf("group member filter = %#v", filter.ScopeIDs)
	}
	if _, err := pool.Exec(ctx, "UPDATE users SET status = 'disabled' WHERE id = $1::uuid", managedUser); err != nil {
		t.Fatalf("disable grouped user: %v", err)
	}
	filter, err = authorizationService.ScopeFilter(ctx, authorization.Subject{UserID: managedUser}, authorization.OrganizationRead)
	if err != nil {
		t.Fatalf("ScopeFilter(disabled grouped user) error = %v", err)
	}
	if len(filter.ScopeIDs) != 0 {
		t.Fatalf("disabled grouped user filter = %#v, want empty", filter.ScopeIDs)
	}
	if _, err := pool.Exec(ctx, "UPDATE users SET status = 'active' WHERE id = $1::uuid", managedUser); err != nil {
		t.Fatalf("restore grouped user: %v", err)
	}

	var platformAdminRoleID string
	for _, role := range roles {
		if role.Name == "PlatformAdmin" {
			platformAdminRoleID = role.ID
		}
	}
	if _, err := management.CreateRoleBinding(ctx, teamAdmin, authorization.GrantRoleInput{SubjectType: "user", SubjectID: managedUser, RoleID: platformAdminRoleID, ScopeID: platform.Scope.ID}, authorizationEvent(teamAdmin)); !errors.Is(err, authorization.ErrGrantNotAllowed) {
		t.Fatalf("team admin platform grant error = %v, want ErrGrantNotAllowed", err)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM audit_events WHERE action IN ('group.create', 'group.member.add', 'role_binding.create')").Scan(&auditCount); err != nil {
		t.Fatalf("count management audit events: %v", err)
	}
	if auditCount != 3 {
		t.Fatalf("management audit events = %d, want 3", auditCount)
	}
}

func authorizationEvent(actorID string) audit.Event {
	return audit.Event{ActorUserID: actorID, RequestID: "t05-test", ClientIP: "192.0.2.50"}
}
