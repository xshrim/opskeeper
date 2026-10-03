//go:build integration

package inspection

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/migrations"
	"opskeeper/backend/notification"
	"opskeeper/backend/secret"
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
	requests := make(chan []byte, 2)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-OpsKeeper-Signature") == "" {
			t.Error("signed webhook request has no signature")
		}
		payload, _ := io.ReadAll(r.Body)
		requests <- payload
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	cipher, err := secret.NewLocalEncryptor(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	configJSON, _ := json.Marshal(map[string]string{"url": server.URL, "signing_secret": "worker-secret"})
	ciphertext, keyVersion, err := cipher.Encrypt(configJSON)
	if err != nil {
		t.Fatal(err)
	}
	configHash := sha256.Sum256(configJSON)
	channelHash := hex.EncodeToString(configHash[:])
	var channelVersion, template, templateVersion, rule string
	if err = pool.QueryRow(ctx, `INSERT INTO notification_channel_versions(scope_id,channel_id,version,provider_config_ciphertext,provider_config_hash,key_version) VALUES($1::uuid,$2::uuid,1,$3,$4,$5) RETURNING id::text`, scope, channel.ID, ciphertext, channelHash, keyVersion).Scan(&channelVersion); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO notification_templates(scope_id,name) VALUES($1::uuid,'transition-test') RETURNING id::text`, scope).Scan(&template); err != nil {
		t.Fatal(err)
	}
	draft, contentHash, err := notification.ValidateTemplate(notification.TemplateDraft{Format: "text", BodyTemplate: "{{.finding_summary}}", Variables: []notification.TemplateVariable{{Name: "finding_summary", Required: true}}})
	if err != nil {
		t.Fatal(err)
	}
	variablesJSON, _ := json.Marshal(draft.Variables)
	if err = pool.QueryRow(ctx, `INSERT INTO notification_template_versions(scope_id,template_id,version,format,body_template,variables,content_hash,status,published_at) VALUES($1::uuid,$2::uuid,1,$3,$4,$5::jsonb,$6,'published',now()) RETURNING id::text`, scope, template, draft.Format, draft.BodyTemplate, variablesJSON, contentHash).Scan(&templateVersion); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO notification_rules(scope_id,name,event_types,filters,aggregation_seconds,max_batch_size) VALUES($1::uuid,'transition-test',ARRAY['finding.opened','finding.reopened','finding.severity_changed','finding.resolved']::text[],jsonb_build_object('resource_ids',jsonb_build_array($2::text),'finding_rules',jsonb_build_array('test.rule')),60,2) RETURNING id::text`, scope, target).Scan(&rule); err != nil {
		t.Fatal(err)
	}
	ruleLinker := NewStore(pool).(interface {
		SetPolicyNotificationRules(context.Context, string, string, []string) ([]string, error)
		ListPolicyNotificationRules(context.Context, string, string) ([]string, error)
	})
	boundRules, err := ruleLinker.SetPolicyNotificationRules(ctx, scope, policy, []string{rule, rule})
	if err != nil || len(boundRules) != 1 || boundRules[0] != rule {
		t.Fatalf("set policy notification rules = %v, %v", boundRules, err)
	}
	listedRules, err := ruleLinker.ListPolicyNotificationRules(ctx, scope, policy)
	if err != nil || len(listedRules) != 1 || listedRules[0] != rule {
		t.Fatalf("list policy notification rules = %v, %v", listedRules, err)
	}
	if _, err := ruleLinker.SetPolicyNotificationRules(ctx, scope, policy, []string{"not-a-uuid"}); err == nil {
		t.Fatal("invalid rule binding succeeded")
	}
	listedRules, err = ruleLinker.ListPolicyNotificationRules(ctx, scope, policy)
	if err != nil || len(listedRules) != 1 || listedRules[0] != rule {
		t.Fatalf("invalid binding partially changed rules = %v, %v", listedRules, err)
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
	if _, err = wrongScopeTx.Exec(ctx, `INSERT INTO inspection_policy_notification_rules(scope_id,policy_id,rule_id,rule_scope_id) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid)`, wrongScope, policy, rule, scope); err == nil {
		_ = wrongScopeTx.Rollback(ctx)
		t.Fatal("cross-Scope policy rule binding succeeded, want foreign-key rejection")
	}
	_ = wrongScopeTx.Rollback(ctx)
	cooldownTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cooldownEvent := notificationEventInput{ScopeID: scope, PolicyID: policy, Type: notification.FindingOpened, IdentityKey: "cooldown-integration", TargetResourceID: target}
	if allowed, err := reserveNotificationRuleCooldown(ctx, cooldownTx, scope, rule, cooldownEvent, time.Hour); err != nil || !allowed {
		_ = cooldownTx.Rollback(ctx)
		t.Fatalf("first cooldown reservation = %t, %v", allowed, err)
	}
	if allowed, err := reserveNotificationRuleCooldown(ctx, cooldownTx, scope, rule, cooldownEvent, time.Hour); err != nil || allowed {
		_ = cooldownTx.Rollback(ctx)
		t.Fatalf("duplicate cooldown reservation = %t, %v; want suppressed", allowed, err)
	}
	if err := cooldownTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
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
	var firstEvent, routeID, channelVersionID, templateVersionID string
	var routeSnapshot []byte
	if err := pool.QueryRow(ctx, `SELECT event.id::text,delivery.route_id::text,delivery.channel_version_id::text,delivery.template_version_id::text,delivery.route_snapshot FROM notification_deliveries delivery JOIN notification_events event ON event.id=delivery.event_id WHERE delivery.scope_id=$1::uuid AND event.event_type='finding.opened' ORDER BY delivery.created_at LIMIT 1`, scope).Scan(&firstEvent, &routeID, &channelVersionID, &templateVersionID, &routeSnapshot); err != nil {
		t.Fatal(err)
	}
	var batchEvent string
	batchPayload, _ := json.Marshal(map[string]string{"event_type": "finding.opened", "severity": "warning", "finding_summary": "second-batch-event"})
	batchHash := sha256.Sum256(batchPayload)
	if err := pool.QueryRow(ctx, `INSERT INTO notification_events(scope_id,event_type,event_key,policy_id,run_id,finding_id,finding_identity,target_resource_id,severity,payload,content_hash) SELECT scope_id,'finding.opened','integration-batch-'||gen_random_uuid()::text,policy_id,run_id,finding_id,'batch-second',target_resource_id,'warning',$2::jsonb,$3 FROM notification_events WHERE id=$1::uuid RETURNING id::text`, firstEvent, batchPayload, hex.EncodeToString(batchHash[:])).Scan(&batchEvent); err != nil {
		t.Fatal(err)
	}
	batchSnapshotHash := sha256.Sum256(routeSnapshot)
	if _, err := pool.Exec(ctx, `INSERT INTO notification_deliveries(scope_id,rule_scope_id,event_id,policy_id,rule_id,route_id,channel_id,channel_version_id,template_version_id,finding_id,run_id,idempotency_key,route_snapshot,snapshot_hash,available_at) SELECT scope_id,rule_scope_id,$2::uuid,policy_id,rule_id,route_id,channel_id,$3::uuid,$4::uuid,finding_id,run_id,$2::text||':'||route_id::text,$5::jsonb,$6,now() FROM notification_deliveries WHERE event_id=$1::uuid`, firstEvent, batchEvent, channelVersionID, templateVersionID, routeSnapshot, hex.EncodeToString(batchSnapshotHash[:])); err != nil {
		t.Fatal(err)
	}
	notifier := NotificationWorker{Store: s, Cipher: cipher, Owner: "integration-worker", LeaseDuration: time.Minute, Sender: WebhookSender{Client: server.Client()}}
	claimed, err := notifier.RunOnce(ctx)
	if err != nil || !claimed {
		t.Fatalf("NotificationWorker.RunOnce() = %t, %v", claimed, err)
	}
	var webhookPayload map[string]string
	if err := json.Unmarshal(<-requests, &webhookPayload); err != nil || !strings.Contains(webhookPayload["body"], "second-batch-event") {
		t.Fatalf("aggregated webhook payload = %v, %v", webhookPayload, err)
	}
	var batchAttemptCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notification_delivery_attempts attempt JOIN notification_deliveries delivery ON delivery.id=attempt.delivery_id WHERE delivery.event_id=ANY(ARRAY[$1::uuid,$2::uuid])`, firstEvent, batchEvent).Scan(&batchAttemptCount); err != nil || batchAttemptCount != 2 {
		t.Fatalf("aggregated attempt count = %d, %v", batchAttemptCount, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE notification_deliveries SET available_at=now() WHERE scope_id=$1::uuid AND status='queued'`, scope); err != nil {
		t.Fatal(err)
	}
	queue := s.(notificationDeliveryQueue)
	abandoned, ok, err := queue.ClaimNotificationDelivery(ctx, "abandoned-worker", time.Minute)
	if err != nil || !ok {
		t.Fatalf("first durable queue claim = %t, %v", ok, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE notification_deliveries SET lease_expires_at=now()-interval '1 second' WHERE id=$1::uuid`, abandoned.ID); err != nil {
		t.Fatal(err)
	}
	reclaimed, ok, err := queue.ClaimNotificationDelivery(ctx, "recovery-worker", time.Minute)
	if err != nil || !ok || reclaimed.ID != abandoned.ID || reclaimed.Items[0].Attempt != 2 {
		t.Fatalf("expired lease recovery = %+v, %t, %v", reclaimed, ok, err)
	}
	if err := queue.FinishNotificationDelivery(ctx, reclaimed, http.StatusTooManyRequests, "try later", 30*time.Second, errors.New("rate limited")); err != nil {
		t.Fatalf("retry failed notification delivery: %v", err)
	}
	var retryStatus string
	var retryAvailable time.Time
	if err := pool.QueryRow(ctx, `SELECT status,available_at FROM notification_deliveries WHERE id=$1::uuid`, abandoned.ID).Scan(&retryStatus, &retryAvailable); err != nil || retryStatus != "queued" || !retryAvailable.After(time.Now()) {
		t.Fatalf("retry queue state=%s at=%s err=%v", retryStatus, retryAvailable, err)
	}
	stats, err := s.(interface {
		NotificationQueueStats(context.Context) (NotificationQueueStats, error)
	}).NotificationQueueStats(ctx)
	if err != nil || stats.Queued != 3 || stats.DeadLetter != 0 {
		t.Fatalf("notification queue stats = %+v, %v", stats, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE notification_deliveries SET available_at=now() WHERE scope_id=$1::uuid AND status='queued'`, scope); err != nil {
		t.Fatal(err)
	}
	startClaims := make(chan struct{})
	claimedJobs := make(chan NotificationDeliveryJob, 2)
	claimErrors := make(chan error, 2)
	for _, owner := range []string{"parallel-one", "parallel-two"} {
		go func(owner string) {
			<-startClaims
			job, ok, err := queue.ClaimNotificationDelivery(ctx, owner, time.Minute)
			if err != nil || !ok {
				claimErrors <- fmt.Errorf("%s claim: claimed=%t err=%w", owner, ok, err)
				return
			}
			claimedJobs <- job
		}(owner)
	}
	close(startClaims)
	parallelA, parallelB := <-claimedJobs, <-claimedJobs
	select {
	case err := <-claimErrors:
		t.Fatal(err)
	default:
	}
	if parallelA.ID == parallelB.ID {
		t.Fatalf("parallel workers claimed the same delivery %s", parallelA.ID)
	}
	for index := range parallelA.Items {
		parallelA.Items[index].MaxAttempts = parallelA.Items[index].Attempt
	}
	if err := queue.FinishNotificationDelivery(ctx, parallelA, http.StatusBadGateway, "unavailable", 0, errors.New("upstream unavailable")); err != nil {
		t.Fatalf("finish exhausted notification delivery: %v", err)
	}
	var deadLetterStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM notification_deliveries WHERE id=$1::uuid`, parallelA.Items[0].ID).Scan(&deadLetterStatus); err != nil || deadLetterStatus != "dead_letter" {
		t.Fatalf("exhausted delivery status = %q, %v", deadLetterStatus, err)
	}
	var attempts int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM notification_delivery_attempts WHERE scope_id=$1::uuid`, scope).Scan(&attempts); err != nil || attempts != 4 {
		t.Fatalf("delivery attempts=%d err=%v, want 4", attempts, err)
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
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM notification_events WHERE scope_id=$1::uuid`, scope).Scan(&events); err != nil || events != 5 {
		t.Fatalf("events after rollback=%d err=%v, want 5", events, err)
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
