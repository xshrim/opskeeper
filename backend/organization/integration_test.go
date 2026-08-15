//go:build integration

package organization_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/migrations"
	"opskeeper/backend/organization"
)

func TestOrganizationLifecycle(t *testing.T) {
	pool := integrationPool(t)
	service := organization.NewService(organization.NewStore(pool))

	platform, err := service.GetPlatform(context.Background())
	if err != nil {
		t.Fatalf("GetPlatform() error = %v", err)
	}
	if platform.Scope.Type != "platform" || platform.Scope.ParentID != nil {
		t.Fatalf("GetPlatform() = %#v", platform)
	}

	team, err := service.CreateTeam(context.Background(), organization.CreateTeamInput{Name: "Payments", Code: "payments"})
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	if team.Scope.ParentID == nil || *team.Scope.ParentID != platform.Scope.ID {
		t.Fatalf("team scope = %#v, platform scope = %#v", team.Scope, platform.Scope)
	}

	project, err := service.CreateProject(context.Background(), organization.CreateProjectInput{
		TeamID: team.ID,
		Name:   "Checkout",
		Code:   "checkout",
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if project.Scope.ParentID == nil || *project.Scope.ParentID != team.Scope.ID {
		t.Fatalf("project scope = %#v, team scope = %#v", project.Scope, team.Scope)
	}

	disabled := organization.StatusDisabled
	updatedTeam, err := service.UpdateTeam(context.Background(), team.ID, organization.UpdateTeamInput{Status: &disabled})
	if err != nil {
		t.Fatalf("UpdateTeam() error = %v", err)
	}
	if updatedTeam.Status != disabled || updatedTeam.Scope.Status != disabled {
		t.Fatalf("UpdateTeam() status = %q, scope status = %q", updatedTeam.Status, updatedTeam.Scope.Status)
	}
	_, err = service.CreateProject(context.Background(), organization.CreateProjectInput{
		TeamID: team.ID,
		Name:   "Ledger",
		Code:   "ledger",
	})
	if !errors.Is(err, organization.ErrParentInactive) {
		t.Fatalf("CreateProject() error = %v, want ErrParentInactive", err)
	}
}

func TestConcurrentTeamCodeIsUnique(t *testing.T) {
	pool := integrationPool(t)
	service := organization.NewService(organization.NewStore(pool))

	var wait sync.WaitGroup
	errorsByCall := make(chan error, 2)
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := service.CreateTeam(context.Background(), organization.CreateTeamInput{Name: "Payments", Code: "payments"})
			errorsByCall <- err
		}()
	}
	wait.Wait()
	close(errorsByCall)

	var succeeded, conflicted int
	for err := range errorsByCall {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, organization.ErrConflict):
			conflicted++
		default:
			t.Fatalf("CreateTeam() unexpected error = %v", err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("concurrent create results: succeeded=%d conflicted=%d", succeeded, conflicted)
	}
}

func TestDatabaseRejectsIllegalScopeHierarchy(t *testing.T) {
	pool := integrationPool(t)
	platform, err := organization.NewService(organization.NewStore(pool)).GetPlatform(context.Background())
	if err != nil {
		t.Fatalf("GetPlatform() error = %v", err)
	}
	_, err = pool.Exec(context.Background(), `
		INSERT INTO scopes (scope_type, parent_scope_id)
		VALUES ('project', $1::uuid)`, platform.Scope.ID)
	if err == nil {
		t.Fatal("illegal project scope parent was accepted")
	}
}

func TestMigrationRollback(t *testing.T) {
	pool := integrationPool(t)
	if err := migrations.RollbackLast(context.Background(), pool); err != nil {
		t.Fatalf("RollbackLast() error = %v", err)
	}
	var usersTable *string
	if err := pool.QueryRow(context.Background(), "SELECT to_regclass(current_schema() || '.users')::text").Scan(&usersTable); err != nil {
		t.Fatalf("check users table: %v", err)
	}
	if usersTable != nil {
		t.Fatalf("users table still exists after identity rollback: %s", *usersTable)
	}
	if err := migrations.RollbackLast(context.Background(), pool); err != nil {
		t.Fatalf("RollbackLast() status migration error = %v", err)
	}
	var organizationStatusColumns int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*)
		  FROM information_schema.columns
		 WHERE table_schema = current_schema()
		   AND table_name IN ('platforms', 'teams', 'projects')
		   AND column_name = 'status'`).Scan(&organizationStatusColumns); err != nil {
		t.Fatalf("check restored status columns: %v", err)
	}
	if organizationStatusColumns != 3 {
		t.Fatalf("restored organization status columns = %d, want 3", organizationStatusColumns)
	}
	if err := migrations.RollbackLast(context.Background(), pool); err != nil {
		t.Fatalf("RollbackLast() organization migration error = %v", err)
	}
	var scopesTable *string
	if err := pool.QueryRow(context.Background(), "SELECT to_regclass(current_schema() || '.scopes')::text").Scan(&scopesTable); err != nil {
		t.Fatalf("check scopes table: %v", err)
	}
	if scopesTable != nil {
		t.Fatalf("scopes table still exists after rollback: %s", *scopesTable)
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
	schema := fmt.Sprintf("opskeeper_test_%d", time.Now().UnixNano())
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
