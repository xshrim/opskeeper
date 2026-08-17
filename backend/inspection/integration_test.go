//go:build integration

package inspection

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/migrations"
)

// TestInspectionTablesExist verifies the deployed T13 schema directly. The
// migration package owns isolated-schema rollback coverage; this test protects
// the feature package from silently running against an incomplete database.
func TestInspectionTablesExist(t *testing.T) {
	url := os.Getenv("OPSK_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("OPSK_TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := migrations.Apply(context.Background(), pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	var count int
	err = pool.QueryRow(context.Background(), `SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name = ANY(ARRAY['inspection_policies','inspection_runs','inspection_jobs','inspection_findings','inspection_health_snapshots','notification_channels','notification_deliveries'])`).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 7 {
		t.Fatalf("T13 table count = %d, want 7", count)
	}
}

func TestLabelSelectorResolvesOnlyMatchingActiveTargets(t *testing.T) {
	url := os.Getenv("OPSK_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("OPSK_TEST_DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var scope, matching, wrong, policy string
	if err = pool.QueryRow(ctx, `INSERT INTO scopes(scope_type) VALUES('platform') RETURNING id::text`).Scan(&scope); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM inspection_policy_targets WHERE policy_id IN (SELECT id FROM inspection_policies WHERE scope_id=$1::uuid); DELETE FROM inspection_jobs WHERE run_id IN (SELECT id FROM inspection_runs WHERE scope_id=$1::uuid); DELETE FROM inspection_runs WHERE scope_id=$1::uuid; DELETE FROM inspection_policies WHERE scope_id=$1::uuid; DELETE FROM resources WHERE scope_id=$1::uuid; DELETE FROM scopes WHERE id=$1::uuid`, scope)
	})
	if err = pool.QueryRow(ctx, `INSERT INTO resources(scope_id,kind,name,labels) VALUES($1::uuid,'Application','matching', '{"env":"prod","team":"payments"}') RETURNING id::text`, scope).Scan(&matching); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO resources(scope_id,kind,name,labels,status) VALUES($1::uuid,'Application','wrong', '{"env":"dev"}','active') RETURNING id::text`, scope).Scan(&wrong); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO inspection_policies(scope_id,name,cron,timezone,target_labels,skill_resource_ids) VALUES($1::uuid,'label-'||gen_random_uuid()::text,'* * * * *','UTC','{"env":"prod"}','{}') RETURNING id::text`, scope).Scan(&policy); err != nil {
		t.Fatal(err)
	}
	targets, err := NewStore(pool).ResolveTargets(ctx, Policy{ID: policy, ScopeID: scope, TargetLabels: map[string]string{"env": "prod"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0] != matching || targets[0] == wrong {
		t.Fatalf("resolved targets=%v, want [%s]", targets, matching)
	}
}

func TestJobLeaseRecoveryAndRetry(t *testing.T) {
	t.Skip("requires an isolated PostgreSQL schema because inspection job claiming is intentionally global")
	url := os.Getenv("OPSK_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("OPSK_TEST_DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := migrations.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	s := NewStore(pool)
	var scope, policy, run string
	if err = pool.QueryRow(ctx, `INSERT INTO scopes(scope_type) VALUES('platform') RETURNING id::text`).Scan(&scope); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM inspection_jobs WHERE run_id IN (SELECT id FROM inspection_runs WHERE scope_id=$1::uuid); DELETE FROM inspection_runs WHERE scope_id=$1::uuid; DELETE FROM inspection_policies WHERE scope_id=$1::uuid; DELETE FROM scopes WHERE id=$1::uuid`, scope)
	})
	if err = pool.QueryRow(ctx, `INSERT INTO inspection_policies(scope_id,name,cron,timezone,skill_resource_ids,timeout_seconds) VALUES($1::uuid,'lease-test-'||gen_random_uuid()::text,'* * * * *','UTC','{}',10) RETURNING id::text`, scope).Scan(&policy); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO inspection_runs(policy_id,scope_id,window_start,window_end,trigger,policy_snapshot,target_snapshot) VALUES($1::uuid,$2::uuid,now(),now()+interval '1 minute','manual','{}','[]') RETURNING id::text`, policy, scope).Scan(&run); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO inspection_jobs(run_id,idempotency_key,max_attempts) VALUES($1::uuid,$2,2)`, run, "lease-"+run); err != nil {
		t.Fatal(err)
	}
	first, ok, err := s.ClaimJob(ctx, "one", time.Minute)
	if err != nil || !ok {
		t.Fatalf("first claim %v %v", ok, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE inspection_jobs SET lease_expires_at=now()-interval '1 second' WHERE id=$1::uuid`, first.ID); err != nil {
		t.Fatal(err)
	}
	second, ok, err := s.ClaimJob(ctx, "two", time.Minute)
	if err != nil || !ok || second.Attempt != 2 {
		t.Fatalf("reclaim %+v %v %v", second, ok, err)
	}
	if err = s.FinishJob(ctx, second.ID, "two", context.DeadlineExceeded); err != nil {
		t.Fatal(err)
	}
	var status string
	if err = pool.QueryRow(ctx, `SELECT status FROM inspection_jobs WHERE id=$1::uuid`, second.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Fatalf("status=%s", status)
	}
	var retryRun string
	if err = pool.QueryRow(ctx, `INSERT INTO inspection_runs(policy_id,scope_id,window_start,window_end,trigger,policy_snapshot,target_snapshot) VALUES($1::uuid,$2::uuid,now()+interval '2 minutes',now()+interval '3 minutes','manual','{}','[]') RETURNING id::text`, policy, scope).Scan(&retryRun); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO inspection_jobs(run_id,idempotency_key,max_attempts) VALUES($1::uuid,$2,2)`, retryRun, "retry-"+retryRun); err != nil {
		t.Fatal(err)
	}
	retry, ok, err := s.ClaimJob(ctx, "three", time.Minute)
	if err != nil || !ok {
		t.Fatalf("retry claim %v %v", ok, err)
	}
	if err = s.FinishJob(ctx, retry.ID, "three", context.DeadlineExceeded); err != nil {
		t.Fatal(err)
	}
	var available time.Time
	if err = pool.QueryRow(ctx, `SELECT available_at FROM inspection_jobs WHERE id=$1::uuid`, retry.ID).Scan(&available); err != nil {
		t.Fatal(err)
	}
	if !available.After(time.Now()) {
		t.Fatal("retry was not delayed")
	}
}

func TestHealthScoreIsDeterministic(t *testing.T) {
	if got := HealthScore([]RuleResult{{Severity: "critical"}, {Severity: "warning"}}); got != 30 {
		t.Fatalf("score=%d want 30", got)
	}
	if got := HealthScore([]RuleResult{{Severity: "critical", Weight: 120}}); got != 0 {
		t.Fatalf("clamped score=%d", got)
	}
}

func TestFindingIsResolvedWhenRuleDisappears(t *testing.T) {
	url := os.Getenv("OPSK_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("OPSK_TEST_DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := migrations.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	s := NewStore(pool)
	var scope, target, policy, run1, run2 string
	if err = pool.QueryRow(ctx, `INSERT INTO scopes(scope_type) VALUES('platform') RETURNING id::text`).Scan(&scope); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM inspection_health_snapshots WHERE policy_id IN (SELECT id FROM inspection_policies WHERE scope_id=$1::uuid); DELETE FROM inspection_findings WHERE policy_id IN (SELECT id FROM inspection_policies WHERE scope_id=$1::uuid); DELETE FROM inspection_jobs WHERE run_id IN (SELECT id FROM inspection_runs WHERE scope_id=$1::uuid); DELETE FROM inspection_runs WHERE scope_id=$1::uuid; DELETE FROM inspection_policies WHERE scope_id=$1::uuid; DELETE FROM resources WHERE scope_id=$1::uuid; DELETE FROM scopes WHERE id=$1::uuid`, scope)
	})
	if err = pool.QueryRow(ctx, `INSERT INTO resources(scope_id,kind,name) VALUES($1::uuid,'PostgreSQL','target-'||gen_random_uuid()::text) RETURNING id::text`, scope).Scan(&target); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO inspection_policies(scope_id,name,cron,timezone,skill_resource_ids,timeout_seconds) VALUES($1::uuid,'finding-'||gen_random_uuid()::text,'* * * * *','UTC','{}',10) RETURNING id::text`, scope).Scan(&policy); err != nil {
		t.Fatal(err)
	}
	makeRun := func(id *string) {
		if err = pool.QueryRow(ctx, `INSERT INTO inspection_runs(policy_id,scope_id,window_start,window_end,trigger,policy_snapshot,target_snapshot) VALUES($1::uuid,$2::uuid,now()+random()*interval '1 hour',now()+interval '2 hours','manual','{}','[]') RETURNING id::text`, policy, scope).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	makeRun(&run1)
	p := Policy{ID: policy, ScopeID: scope, TargetResourceIDs: []string{target}}
	if _, snapshots, err := s.SaveResults(ctx, Run{ID: run1, WindowStart: time.Now()}, p, []RuleResult{{TargetResourceID: target, Rule: "test.rule", Severity: "warning"}}); err != nil || len(snapshots) != 1 || snapshots[0].Score != 80 {
		t.Fatalf("first save %v %+v", err, snapshots)
	}
	makeRun(&run2)
	if _, _, err = s.SaveResults(ctx, Run{ID: run2, WindowStart: time.Now().Add(time.Minute)}, p, nil); err != nil {
		t.Fatal(err)
	}
	var status string
	if err = pool.QueryRow(ctx, `SELECT status FROM inspection_findings WHERE policy_id=$1::uuid`, policy).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "resolved" {
		t.Fatalf("finding status=%s", status)
	}
}
