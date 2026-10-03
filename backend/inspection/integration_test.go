//go:build integration

package inspection

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
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
	if err = pool.QueryRow(ctx, `INSERT INTO resources(scope_id,kind,name,labels) VALUES($1::uuid,'Host','matching', '{"env":"prod","team":"payments"}') RETURNING id::text`, scope).Scan(&matching); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO resources(scope_id,kind,name,labels,status) VALUES($1::uuid,'Host','wrong', '{"env":"dev"}','active') RETURNING id::text`, scope).Scan(&wrong); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO inspection_policies(scope_id,name,cron,timezone,target_labels) VALUES($1::uuid,'label-'||gen_random_uuid()::text,'* * * * *','UTC','{"env":"prod"}') RETURNING id::text`, scope).Scan(&policy); err != nil {
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
	if err = pool.QueryRow(ctx, `INSERT INTO inspection_policies(scope_id,name,cron,timezone,timeout_seconds) VALUES($1::uuid,'lease-test-'||gen_random_uuid()::text,'* * * * *','UTC',10) RETURNING id::text`, scope).Scan(&policy); err != nil {
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

func TestFindingNotificationEventsAreTransactional(t *testing.T) {
	ctx := context.Background()
	pool := inspectionIntegrationPool(t)
	var err error
	if err := migrations.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	s := NewStore(pool)
	var scope, target, policy, run1, run2 string
	if err = pool.QueryRow(ctx, `INSERT INTO scopes(scope_type) VALUES('platform') RETURNING id::text`).Scan(&scope); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO resources(scope_id,kind,name) VALUES($1::uuid,'PostgreSQL','target-'||gen_random_uuid()::text) RETURNING id::text`, scope).Scan(&target); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO inspection_policies(scope_id,name,cron,timezone,timeout_seconds) VALUES($1::uuid,'finding-'||gen_random_uuid()::text,'* * * * *','UTC',10) RETURNING id::text`, scope).Scan(&policy); err != nil {
		t.Fatal(err)
	}
	channel, err := s.CreateChannel(ctx, NotificationChannel{ScopeID: scope, Name: "transition-test", Kind: "webhook", WebhookURL: "https://example.test/notify", Status: "active", RateLimitPerMinute: 30})
	if err != nil {
		t.Fatal(err)
	}
	channelHash := strings.Repeat("a", 64)
	var channelVersion, template, templateVersion, rule string
	if err = pool.QueryRow(ctx, `INSERT INTO notification_channel_versions(scope_id,channel_id,version,provider_config_hash) VALUES($1::uuid,$2::uuid,1,$3) RETURNING id::text`, scope, channel.ID, channelHash).Scan(&channelVersion); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO notification_templates(scope_id,name) VALUES($1::uuid,'transition-test') RETURNING id::text`, scope).Scan(&template); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO notification_template_versions(scope_id,template_id,version,format,body_template,content_hash,status,published_at) VALUES($1::uuid,$2::uuid,1,'text','{{finding_summary}}',$3,'published',now()) RETURNING id::text`, scope, template, strings.Repeat("b", 64)).Scan(&templateVersion); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO notification_rules(scope_id,name,event_types) VALUES($1::uuid,'transition-test',ARRAY['finding.opened','finding.reopened','finding.severity_changed','finding.resolved']::text[]) RETURNING id::text`, scope).Scan(&rule); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO inspection_policy_notification_rules(scope_id,policy_id,rule_id) VALUES($1::uuid,$2::uuid,$3::uuid)`, scope, policy, rule); err != nil {
		t.Fatal(err)
	}
	wrongScopeTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var wrongScope string
	if err = wrongScopeTx.QueryRow(ctx, `INSERT INTO scopes(scope_type) VALUES('platform') RETURNING id::text`).Scan(&wrongScope); err != nil {
		_ = wrongScopeTx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err = wrongScopeTx.Exec(ctx, `INSERT INTO inspection_policy_notification_rules(scope_id,policy_id,rule_id) VALUES($1::uuid,$2::uuid,$3::uuid)`, wrongScope, policy, rule); err == nil {
		_ = wrongScopeTx.Rollback(ctx)
		t.Fatal("cross-Scope policy rule binding succeeded, want foreign-key rejection")
	}
	_ = wrongScopeTx.Rollback(ctx)
	if _, err = pool.Exec(ctx, `INSERT INTO notification_rule_routes(scope_id,rule_id,channel_id,channel_version_id,template_version_id) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid)`, scope, rule, channel.ID, channelVersion, templateVersion); err != nil {
		t.Fatal(err)
	}
	makeRun := func(id *string, offset time.Duration) {
		start := time.Now().UTC().Add(offset)
		if err = pool.QueryRow(ctx, `INSERT INTO inspection_runs(policy_id,scope_id,window_start,window_end,trigger,policy_snapshot,target_snapshot) VALUES($1::uuid,$2::uuid,$3::timestamptz,$3::timestamptz+interval '1 second','manual','{}','[]') RETURNING id::text`, policy, scope, start).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	makeRun(&run1, 0)
	p := Policy{ID: policy, ScopeID: scope, TargetResourceIDs: []string{target}}
	if _, snapshots, err := s.SaveResults(ctx, Run{ID: run1, WindowStart: time.Now()}, p, []RuleResult{{TargetResourceID: target, Rule: "test.rule", Severity: "warning"}}); err != nil || len(snapshots) != 1 || snapshots[0].Score != 80 {
		t.Fatalf("first save %v %+v", err, snapshots)
	}
	if _, _, err = s.SaveResults(ctx, Run{ID: run1, WindowStart: time.Now()}, p, []RuleResult{{TargetResourceID: target, Rule: "test.rule", Severity: "warning"}}); err != nil {
		t.Fatalf("repeat first run: %v", err)
	}
	makeRun(&run2, time.Minute)
	if _, _, err = s.SaveResults(ctx, Run{ID: run2, WindowStart: time.Now().Add(time.Minute)}, p, []RuleResult{{TargetResourceID: target, Rule: "test.rule", Severity: "warning"}}); err != nil {
		t.Fatal(err)
	}
	var run3 string
	makeRun(&run3, 2*time.Minute)
	if _, _, err = s.SaveResults(ctx, Run{ID: run3, WindowStart: time.Now().Add(2 * time.Minute)}, p, []RuleResult{{TargetResourceID: target, Rule: "test.rule", Severity: "critical"}}); err != nil {
		t.Fatal(err)
	}
	var run4 string
	makeRun(&run4, 3*time.Minute)
	if _, _, err = s.SaveResults(ctx, Run{ID: run4, WindowStart: time.Now().Add(3 * time.Minute)}, p, nil); err != nil {
		t.Fatal(err)
	}
	var run5 string
	makeRun(&run5, 4*time.Minute)
	if _, _, err = s.SaveResults(ctx, Run{ID: run5, WindowStart: time.Now().Add(4 * time.Minute)}, p, []RuleResult{{TargetResourceID: target, Rule: "test.rule", Severity: "warning", Message: "token=not-for-notification"}}); err != nil {
		t.Fatal(err)
	}
	var status string
	if err = pool.QueryRow(ctx, `SELECT status FROM inspection_findings WHERE policy_id=$1::uuid`, policy).Scan(&status); err != nil || status != "open" {
		t.Fatalf("finding status=%s err=%v, want open", status, err)
	}
	var events, deliveries int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM notification_events WHERE scope_id=$1::uuid`, scope).Scan(&events); err != nil || events != 4 {
		t.Fatalf("event count=%d err=%v, want 4 transitions", events, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM notification_deliveries WHERE scope_id=$1::uuid AND event_id IS NOT NULL`, scope).Scan(&deliveries); err != nil || deliveries != 4 {
		t.Fatalf("delivery count=%d err=%v, want 4 routed events", deliveries, err)
	}
	var types []string
	if err = pool.QueryRow(ctx, `SELECT array_agg(event_type ORDER BY occurred_at,event_type) FROM notification_events WHERE scope_id=$1::uuid`, scope).Scan(&types); err != nil {
		t.Fatal(err)
	}
	if strings.Join(types, ",") != "finding.opened,finding.severity_changed,finding.resolved,finding.reopened" {
		t.Fatalf("event types=%v", types)
	}
	var summary string
	if err = pool.QueryRow(ctx, `SELECT payload->>'finding_summary' FROM notification_events WHERE scope_id=$1::uuid AND event_type='finding.reopened'`, scope).Scan(&summary); err != nil || strings.Contains(summary, "not-for-notification") {
		t.Fatalf("notification summary=%q err=%v, secret should be redacted", summary, err)
	}
	var deliveryID string
	if err = pool.QueryRow(ctx, `SELECT id::text FROM notification_deliveries WHERE scope_id=$1::uuid AND event_id IS NOT NULL ORDER BY created_at LIMIT 1`, scope).Scan(&deliveryID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE notification_deliveries SET route_snapshot='{"changed":true}'::jsonb WHERE id=$1::uuid`, deliveryID); err == nil {
		t.Fatal("route snapshot update succeeded, want immutable snapshot rejection")
	}
	delivery, claimedChannel, claimed, err := s.ClaimDelivery(ctx)
	if err != nil || !claimed {
		t.Fatalf("ClaimDelivery() = claimed %t, err %v", claimed, err)
	}
	if delivery.ScopeID != scope || delivery.EventType == "" || len(delivery.EventPayload) == 0 || delivery.MaxAttempts != 5 || claimedChannel.ID != channel.ID {
		t.Fatalf("claimed delivery=%+v channel=%+v", delivery, claimedChannel)
	}
	if err := s.FinishDelivery(ctx, delivery, 204, "accepted", nil); err != nil {
		t.Fatalf("FinishDelivery(): %v", err)
	}
	var attempts int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM notification_delivery_attempts WHERE scope_id=$1::uuid`, scope).Scan(&attempts); err != nil || attempts != 1 {
		t.Fatalf("delivery attempts=%d err=%v, want 1", attempts, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE notification_delivery_attempts SET status='failed' WHERE scope_id=$1::uuid`, scope); err == nil {
		t.Fatal("delivery attempt update succeeded, want append-only rejection")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM notification_delivery_attempts WHERE scope_id=$1::uuid`, scope); err == nil {
		t.Fatal("delivery attempt deletion succeeded, want append-only rejection")
	}
	makeRun(&run2, 5*time.Minute)
	var invalidTarget string
	if err = pool.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&invalidTarget); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.SaveResults(ctx, Run{ID: run2, WindowStart: time.Now().Add(5 * time.Minute)}, p, []RuleResult{
		{TargetResourceID: target, Rule: "transaction.rollback", Severity: "warning"},
		{TargetResourceID: invalidTarget, Rule: "transaction.rollback", Severity: "warning"},
	}); err == nil {
		t.Fatal("SaveResults() succeeded with an invalid resource, want transaction rollback")
	}
	var partial int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM inspection_findings WHERE policy_id=$1::uuid AND rule='transaction.rollback'`, policy).Scan(&partial); err != nil || partial != 0 {
		t.Fatalf("partially committed findings=%d err=%v", partial, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM notification_events WHERE scope_id=$1::uuid`, scope).Scan(&events); err != nil || events != 4 {
		t.Fatalf("events after rollback=%d err=%v, want 4", events, err)
	}
	var failedRun, failedJob string
	makeRun(&failedRun, 6*time.Minute)
	if err = pool.QueryRow(ctx, `INSERT INTO inspection_jobs(run_id,idempotency_key,max_attempts,status,attempt,lease_owner) VALUES($1::uuid,$2,1,'leased',1,'integration') RETURNING id::text`, failedRun, "failed-"+failedRun).Scan(&failedJob); err != nil {
		t.Fatal(err)
	}
	if err = s.FinishJob(ctx, failedJob, "integration", errors.New("worker failed")); err != nil {
		t.Fatal(err)
	}
	var degradedRun string
	makeRun(&degradedRun, 7*time.Minute)
	if _, err = pool.Exec(ctx, `UPDATE inspection_runs SET deterministic_completed=true WHERE id=$1::uuid`, degradedRun); err != nil {
		t.Fatal(err)
	}
	if err = s.MarkLLMStatus(ctx, degradedRun, "degraded"); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM notification_events WHERE scope_id=$1::uuid AND event_type IN ('inspection.failed','inspection.degraded')`, scope).Scan(&events); err != nil || events != 2 {
		t.Fatalf("inspection lifecycle events=%d err=%v, want failed and degraded", events, err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM notification_events WHERE scope_id=$1::uuid AND event_type='inspection.degraded'`, scope); err == nil {
		t.Fatal("notification event deletion succeeded, want immutable event rejection")
	}
}

func inspectionIntegrationPool(t *testing.T) *pgxpool.Pool {
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
	schema := fmt.Sprintf("inspection_test_%d", time.Now().UnixNano())
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
	config.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		_, _ = adminPool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		adminPool.Close()
		t.Fatalf("connect integration schema: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		if _, err := adminPool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop integration schema: %v", err)
		}
		adminPool.Close()
	})
	return pool
}
