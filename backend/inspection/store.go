package inspection

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
)

type Store interface {
	CreatePolicy(context.Context, Policy, string) (Policy, error)
	ListPolicies(context.Context, string) ([]Policy, error)
	ScheduleDue(context.Context, time.Time) (int, error)
	CreateScheduledRun(context.Context, Policy, time.Time, time.Time, []string) (string, bool, error)
	ClaimJob(context.Context, string, time.Duration) (Job, bool, error)
	Heartbeat(context.Context, string, string, time.Duration) (bool, error)
	FinishJob(context.Context, string, string, error) error
	GetRun(context.Context, string) (Run, Policy, []string, error)
	StartRun(context.Context, string) error
	SaveResults(context.Context, Run, Policy, []RuleResult) ([]Finding, []HealthSnapshot, error)
	CreateManualRun(context.Context, Policy, time.Time, []string) (string, error)
	ListRuns(context.Context, string, int) ([]Run, error)
	ListFindings(context.Context, string, int) ([]Finding, error)
	CreateChannel(context.Context, NotificationChannel) (NotificationChannel, error)
	ListChannels(context.Context, string) ([]NotificationChannel, error)
	MarkLLMStatus(context.Context, string, string) error
	EnqueueDeliveries(context.Context, string, []Finding) error
	ClaimDelivery(context.Context) (Delivery, NotificationChannel, bool, error)
	FinishDelivery(context.Context, Delivery, int, string, error) error
	GetFinding(context.Context, string) (Finding, error)
	SetPolicyStatus(context.Context, string, string, string) error
}

type Job struct {
	ID, RunID, LeaseOwner string
	Attempt, MaxAttempts  int
}

type store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) Store { return &store{pool: pool} }

func (s *store) CreatePolicy(ctx context.Context, input Policy, actorID string) (Policy, error) {
	labels, err := json.Marshal(input.TargetLabels)
	if err != nil {
		return Policy{}, err
	}
	windows, err := json.Marshal(input.Maintenance)
	if err != nil {
		return Policy{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Policy{}, err
	}
	defer tx.Rollback(ctx)
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO inspection_policies (scope_id,name,cron,timezone,status,target_labels,skill_resource_ids,timeout_seconds,retries,max_concurrent,max_tool_calls,max_tokens,maintenance_windows,created_by) VALUES ($1::uuid,$2,$3,$4,$5,$6,$7::uuid[],$8,$9,$10,$11,$12,$13,NULLIF($14,'')::uuid) RETURNING id::text`, input.ScopeID, input.Name, input.Cron, input.Timezone, input.Status, labels, input.SkillResourceIDs, int(input.Timeout.Seconds()), input.Retries, input.MaxConcurrent, input.MaxToolCalls, input.MaxTokens, windows, actorID).Scan(&id)
	if err != nil {
		return Policy{}, mapError(err)
	}
	for _, target := range input.TargetResourceIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO inspection_policy_targets (policy_id,resource_id) VALUES ($1::uuid,$2::uuid)`, id, target); err != nil {
			return Policy{}, mapError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Policy{}, err
	}
	input.ID = id
	return input, nil
}

func (s *store) ListPolicies(ctx context.Context, scopeID string) ([]Policy, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,scope_id::text,name,cron,timezone,status,target_labels,skill_resource_ids::text[],timeout_seconds,retries,max_concurrent,max_tool_calls,max_tokens,maintenance_windows FROM inspection_policies WHERE scope_id=$1::uuid AND deleted_at IS NULL ORDER BY created_at DESC`, scopeID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := []Policy{}
	for rows.Next() {
		var item Policy
		var labels, windows []byte
		var seconds int
		if err := rows.Scan(&item.ID, &item.ScopeID, &item.Name, &item.Cron, &item.Timezone, &item.Status, &labels, &item.SkillResourceIDs, &seconds, &item.Retries, &item.MaxConcurrent, &item.MaxToolCalls, &item.MaxTokens, &windows); err != nil {
			return nil, err
		}
		item.Timeout = time.Duration(seconds) * time.Second
		_ = json.Unmarshal(labels, &item.TargetLabels)
		_ = json.Unmarshal(windows, &item.Maintenance)
		if err := s.pool.QueryRow(ctx, `SELECT COALESCE(array_agg(resource_id::text ORDER BY resource_id::text),'{}') FROM inspection_policy_targets WHERE policy_id=$1::uuid`, item.ID).Scan(&item.TargetResourceIDs); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ScheduleDue is protected by a PostgreSQL advisory lock, so multiple
// schedulers may poll safely. The unique policy/window constraint remains the
// second line of defense if a process is interrupted after planning a run.
func (s *store) ScheduleDue(ctx context.Context, now time.Time) (int, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Release()
	const lockID int64 = 0x4f50534b494e5350
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, lockID).Scan(&locked); err != nil || !locked {
		return 0, err
	}
	defer conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, lockID)
	rows, err := conn.Query(ctx, `SELECT id::text,scope_id::text,name,cron,timezone,status,target_labels,skill_resource_ids::text[],timeout_seconds,retries,max_concurrent,max_tool_calls,max_tokens,maintenance_windows FROM inspection_policies WHERE status='active' AND deleted_at IS NULL`)
	if err != nil {
		return 0, mapError(err)
	}
	defer rows.Close()
	created := 0
	for rows.Next() {
		policy, err := scanPolicy(rows)
		if err != nil {
			return created, err
		}
		location, err := time.LoadLocation(policy.Timezone)
		if err != nil {
			continue
		}
		schedule, err := cron.ParseStandard(policy.Cron)
		if err != nil {
			continue
		}
		localNow := now.In(location).Truncate(time.Minute)
		if next := schedule.Next(localNow.Add(-time.Minute)); next.After(localNow) || next.Before(localNow) || IsMaintenance(policy.Maintenance, localNow) {
			continue
		}
		if err := conn.QueryRow(ctx, `SELECT COALESCE(array_agg(resource_id::text ORDER BY resource_id::text),'{}') FROM inspection_policy_targets WHERE policy_id=$1::uuid`, policy.ID).Scan(&policy.TargetResourceIDs); err != nil {
			return created, err
		}
		if len(policy.TargetResourceIDs) == 0 {
			continue
		}
		if _, ok, err := s.CreateScheduledRun(ctx, policy, localNow.UTC(), localNow.UTC().Add(time.Minute), policy.TargetResourceIDs); err != nil {
			return created, err
		} else if ok {
			created++
		}
	}
	return created, rows.Err()
}

type policyScanner interface{ Scan(...any) error }

func scanPolicy(row policyScanner) (Policy, error) {
	var item Policy
	var labels, windows []byte
	var seconds int
	if err := row.Scan(&item.ID, &item.ScopeID, &item.Name, &item.Cron, &item.Timezone, &item.Status, &labels, &item.SkillResourceIDs, &seconds, &item.Retries, &item.MaxConcurrent, &item.MaxToolCalls, &item.MaxTokens, &windows); err != nil {
		return Policy{}, err
	}
	item.Timeout = time.Duration(seconds) * time.Second
	_ = json.Unmarshal(labels, &item.TargetLabels)
	_ = json.Unmarshal(windows, &item.Maintenance)
	return item, nil
}

func (s *store) CreateScheduledRun(ctx context.Context, policy Policy, start, end time.Time, targets []string) (string, bool, error) {
	policyRaw, _ := json.Marshal(policy)
	targetRaw, _ := json.Marshal(targets)
	key := policy.ID + ":" + start.UTC().Format(time.RFC3339)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback(ctx)
	var runID string
	err = tx.QueryRow(ctx, `INSERT INTO inspection_runs (policy_id,scope_id,window_start,window_end,trigger,policy_snapshot,target_snapshot) VALUES ($1::uuid,$2::uuid,$3,$4,'schedule',$5,$6) ON CONFLICT (policy_id,window_start) DO NOTHING RETURNING id::text`, policy.ID, policy.ScopeID, start, end, policyRaw, targetRaw).Scan(&runID)
	if err == pgx.ErrNoRows {
		return "", false, tx.Commit(ctx)
	}
	if err != nil {
		return "", false, mapError(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO inspection_jobs (run_id,idempotency_key,max_attempts) VALUES ($1::uuid,$2,$3)`, runID, key, policy.Retries+1)
	if err != nil {
		return "", false, mapError(err)
	}
	return runID, true, tx.Commit(ctx)
}

func (s *store) ClaimJob(ctx context.Context, owner string, lease time.Duration) (Job, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Job{}, false, err
	}
	defer tx.Rollback(ctx)
	var job Job
	err = tx.QueryRow(ctx, `WITH next AS (SELECT id FROM inspection_jobs WHERE (status='queued' AND available_at<=now()) OR (status='leased' AND lease_expires_at<now()) ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE inspection_jobs job SET status='leased',attempt=attempt+1,lease_owner=$1,lease_expires_at=now()+$2::interval,heartbeat_at=now(),updated_at=now() FROM next WHERE job.id=next.id RETURNING job.id::text,job.run_id::text,job.lease_owner,job.attempt,job.max_attempts`, owner, lease.String()).Scan(&job.ID, &job.RunID, &job.LeaseOwner, &job.Attempt, &job.MaxAttempts)
	if err == pgx.ErrNoRows {
		return Job{}, false, tx.Commit(ctx)
	}
	if err != nil {
		return Job{}, false, mapError(err)
	}
	return job, true, tx.Commit(ctx)
}

func (s *store) Heartbeat(ctx context.Context, id, owner string, lease time.Duration) (bool, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE inspection_jobs SET heartbeat_at=now(),lease_expires_at=now()+$3::interval,updated_at=now() WHERE id=$1::uuid AND status='leased' AND lease_owner=$2`, id, owner, lease.String())
	return tag.RowsAffected() == 1, mapError(err)
}
func (s *store) FinishJob(ctx context.Context, id, owner string, runErr error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var attempt, maxAttempts int
	var runID string
	err = tx.QueryRow(ctx, `SELECT run_id::text,attempt,max_attempts FROM inspection_jobs WHERE id=$1::uuid AND lease_owner=$2 AND status='leased' FOR UPDATE`, id, owner).Scan(&runID, &attempt, &maxAttempts)
	if err != nil {
		return ErrConflict
	}
	if runErr != nil && attempt < maxAttempts {
		delay := time.Duration(1<<(attempt-1)) * time.Second
		_, err = tx.Exec(ctx, `UPDATE inspection_jobs SET status='queued', lease_owner='', lease_expires_at=NULL, heartbeat_at=NULL, available_at=now()+$2::interval,error_code='worker',error_message=$3,updated_at=now() WHERE id=$1::uuid`, id, delay.String(), errorText(runErr))
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	status, runStatus, code := "succeeded", "succeeded", ""
	if runErr != nil {
		status, runStatus, code = "failed", "failed", "worker"
	}
	if _, err = tx.Exec(ctx, `UPDATE inspection_jobs SET status=$2,completed_at=now(),lease_expires_at=NULL,error_code=$3,error_message=$4,updated_at=now() WHERE id=$1::uuid`, id, status, code, errorText(runErr)); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE inspection_runs SET status=$2,completed_at=now(),updated_at=now() WHERE id=$1::uuid`, runID, runStatus); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *store) CreateManualRun(ctx context.Context, policy Policy, now time.Time, targets []string) (string, error) {
	// Manual runs use a nanosecond window to remain independently auditable;
	// scheduled runs retain their stable one-minute scheduling key.
	policyRaw, _ := json.Marshal(policy)
	targetRaw, _ := json.Marshal(targets)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var runID string
	if err = tx.QueryRow(ctx, `INSERT INTO inspection_runs (policy_id,scope_id,window_start,window_end,trigger,policy_snapshot,target_snapshot) VALUES ($1::uuid,$2::uuid,$3,$4,'manual',$5,$6) RETURNING id::text`, policy.ID, policy.ScopeID, now.UTC(), now.UTC().Add(time.Nanosecond), policyRaw, targetRaw).Scan(&runID); err != nil {
		return "", mapError(err)
	}
	key := policy.ID + ":manual:" + now.UTC().Format(time.RFC3339Nano)
	if _, err = tx.Exec(ctx, `INSERT INTO inspection_jobs (run_id,idempotency_key,max_attempts) VALUES ($1::uuid,$2,$3)`, runID, key, policy.Retries+1); err != nil {
		return "", mapError(err)
	}
	return runID, tx.Commit(ctx)
}

func (s *store) GetRun(ctx context.Context, id string) (Run, Policy, []string, error) {
	var run Run
	var policyRaw, targetsRaw []byte
	var score *int
	err := s.pool.QueryRow(ctx, `SELECT id::text,policy_id::text,scope_id::text,trigger,status,window_start,window_end,score,deterministic_completed,llm_status,error_code,error_message,started_at,completed_at,policy_snapshot,target_snapshot FROM inspection_runs WHERE id=$1::uuid`, id).Scan(&run.ID, &run.PolicyID, &run.ScopeID, &run.Trigger, &run.Status, &run.WindowStart, &run.WindowEnd, &score, &run.DeterministicCompleted, &run.LLMStatus, &run.ErrorCode, &run.ErrorMessage, &run.StartedAt, &run.CompletedAt, &policyRaw, &targetsRaw)
	if err == pgx.ErrNoRows {
		return Run{}, Policy{}, nil, ErrNotFound
	}
	if err != nil {
		return Run{}, Policy{}, nil, mapError(err)
	}
	run.Score = score
	var policy Policy
	if err := json.Unmarshal(policyRaw, &policy); err != nil {
		return Run{}, Policy{}, nil, err
	}
	var targets []string
	if err := json.Unmarshal(targetsRaw, &targets); err != nil {
		return Run{}, Policy{}, nil, err
	}
	return run, policy, targets, nil
}

func (s *store) StartRun(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE inspection_runs SET status='running',started_at=COALESCE(started_at,now()),updated_at=now() WHERE id=$1::uuid AND status IN ('queued','running')`, id)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func (s *store) SaveResults(ctx context.Context, run Run, policy Policy, results []RuleResult) ([]Finding, []HealthSnapshot, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)
	byTarget := make(map[string][]RuleResult)
	for _, result := range results {
		if result.TargetResourceID != "" {
			byTarget[result.TargetResourceID] = append(byTarget[result.TargetResourceID], result)
		}
	}
	findings := make([]Finding, 0, len(results))
	seen := map[string]bool{}
	for _, result := range results {
		if result.TargetResourceID == "" || result.Rule == "" {
			continue
		}
		identity := FindingIdentityKey(result.TargetResourceID, result.Rule)
		fingerprint := FindingFingerprint(result.TargetResourceID, result.Rule, run.WindowStart)
		var finding Finding
		err := tx.QueryRow(ctx, `INSERT INTO inspection_findings (policy_id,target_resource_id,rule,identity_key,fingerprint,severity,message,status,first_observed_at,last_observed_at,last_run_id,resolved_at) VALUES ($1::uuid,$2::uuid,$3,$4,$5,$6,$7,'open',now(),now(),$8::uuid,NULL) ON CONFLICT (policy_id,identity_key) DO UPDATE SET fingerprint=EXCLUDED.fingerprint,severity=EXCLUDED.severity,message=EXCLUDED.message,status='open',last_observed_at=now(),last_run_id=EXCLUDED.last_run_id,resolved_at=NULL RETURNING id::text,policy_id::text,target_resource_id::text,rule,identity_key,fingerprint,severity,message,status,first_observed_at,last_observed_at,resolved_at`, policy.ID, result.TargetResourceID, result.Rule, identity, fingerprint, result.Severity, result.Message, run.ID).Scan(&finding.ID, &finding.PolicyID, &finding.TargetResourceID, &finding.Rule, &finding.IdentityKey, &finding.Fingerprint, &finding.Severity, &finding.Message, &finding.Status, &finding.FirstObservedAt, &finding.LastObservedAt, &finding.ResolvedAt)
		if err != nil {
			return nil, nil, mapError(err)
		}
		findings = append(findings, finding)
		seen[identity] = true
	}
	// A successful deterministic observation that no longer returns a known
	// rule is a recovery. Scope it to frozen targets, never to a changing policy.
	for _, target := range policy.TargetResourceIDs {
		if _, err := tx.Exec(ctx, `UPDATE inspection_findings SET status='resolved',resolved_at=now(),last_observed_at=now(),last_run_id=$3::uuid WHERE policy_id=$1::uuid AND target_resource_id=$2::uuid AND status='open' AND identity_key <> ALL($4::text[])`, policy.ID, target, run.ID, keys(seen)); err != nil {
			return nil, nil, mapError(err)
		}
	}
	snapshots := make([]HealthSnapshot, 0, len(policy.TargetResourceIDs))
	for _, target := range policy.TargetResourceIDs {
		reasons := byTarget[target]
		if reasons == nil {
			reasons = []RuleResult{}
		}
		score := HealthScore(reasons)
		raw, _ := json.Marshal(reasons)
		if _, err := tx.Exec(ctx, `INSERT INTO inspection_health_snapshots (run_id,policy_id,target_resource_id,score,reasons) VALUES ($1::uuid,$2::uuid,$3::uuid,$4,$5::jsonb)`, run.ID, policy.ID, target, score, string(raw)); err != nil {
			return nil, nil, mapError(err)
		}
		snapshots = append(snapshots, HealthSnapshot{PolicyID: policy.ID, TargetResourceID: target, Score: score, CollectedAt: time.Now().UTC(), Reasons: reasons})
	}
	overall := 100
	for _, snapshot := range snapshots {
		if snapshot.Score < overall {
			overall = snapshot.Score
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE inspection_runs SET score=$2,deterministic_completed=true,updated_at=now() WHERE id=$1::uuid`, run.ID, overall); err != nil {
		return nil, nil, mapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	if err := s.EnqueueDeliveries(ctx, run.ID, findings); err != nil {
		return nil, nil, err
	}
	return findings, snapshots, nil
}

func keys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	return out
}

func (s *store) ListRuns(ctx context.Context, scopeID string, limit int) ([]Run, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,policy_id::text,scope_id::text,trigger,status,window_start,window_end,score,deterministic_completed,llm_status,error_code,error_message,started_at,completed_at FROM inspection_runs WHERE scope_id=$1::uuid ORDER BY created_at DESC LIMIT $2`, scopeID, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		var r Run
		if err := rows.Scan(&r.ID, &r.PolicyID, &r.ScopeID, &r.Trigger, &r.Status, &r.WindowStart, &r.WindowEnd, &r.Score, &r.DeterministicCompleted, &r.LLMStatus, &r.ErrorCode, &r.ErrorMessage, &r.StartedAt, &r.CompletedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *store) ListFindings(ctx context.Context, scopeID string, limit int) ([]Finding, error) {
	rows, err := s.pool.Query(ctx, `SELECT f.id::text,f.policy_id::text,f.target_resource_id::text,f.rule,f.identity_key,f.fingerprint,f.severity,f.message,f.status,f.first_observed_at,f.last_observed_at,f.resolved_at FROM inspection_findings f JOIN inspection_policies p ON p.id=f.policy_id WHERE p.scope_id=$1::uuid ORDER BY f.last_observed_at DESC LIMIT $2`, scopeID, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := []Finding{}
	for rows.Next() {
		var f Finding
		if err := rows.Scan(&f.ID, &f.PolicyID, &f.TargetResourceID, &f.Rule, &f.IdentityKey, &f.Fingerprint, &f.Severity, &f.Message, &f.Status, &f.FirstObservedAt, &f.LastObservedAt, &f.ResolvedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *store) CreateChannel(ctx context.Context, item NotificationChannel) (NotificationChannel, error) {
	err := s.pool.QueryRow(ctx, `INSERT INTO notification_channels(scope_id,name,kind,webhook_url,credential_id,status,rate_limit_per_minute) VALUES ($1::uuid,$2,'webhook',$3,NULLIF($4,'')::uuid,$5,$6) RETURNING id::text`, item.ScopeID, item.Name, item.WebhookURL, credentialID(item.CredentialID), item.Status, item.RateLimitPerMinute).Scan(&item.ID)
	if err != nil {
		return NotificationChannel{}, mapError(err)
	}
	item.Kind = "webhook"
	return item, nil
}
func credentialID(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func (s *store) ListChannels(ctx context.Context, scopeID string) ([]NotificationChannel, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,scope_id::text,name,kind,webhook_url,credential_id::text,status,rate_limit_per_minute FROM notification_channels WHERE scope_id=$1::uuid AND deleted_at IS NULL ORDER BY created_at DESC`, scopeID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := []NotificationChannel{}
	for rows.Next() {
		var n NotificationChannel
		if err := rows.Scan(&n.ID, &n.ScopeID, &n.Name, &n.Kind, &n.WebhookURL, &n.CredentialID, &n.Status, &n.RateLimitPerMinute); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *store) MarkLLMStatus(ctx context.Context, runID, status string) error {
	if status != "succeeded" && status != "degraded" && status != "failed" && status != "not_requested" {
		return invalid("invalid LLM status")
	}
	_, err := s.pool.Exec(ctx, `UPDATE inspection_runs SET llm_status=$2,updated_at=now() WHERE id=$1::uuid`, runID, status)
	return mapError(err)
}

func (s *store) EnqueueDeliveries(ctx context.Context, runID string, findings []Finding) error {
	for _, finding := range findings {
		// The observation fingerprint makes open/reopen notifications idempotent
		// per channel and scheduling window.
		_, err := s.pool.Exec(ctx, `INSERT INTO notification_deliveries(channel_id,finding_id,run_id,idempotency_key) SELECT id,$1::uuid,$2::uuid,id::text||':'||$3 FROM notification_channels WHERE scope_id=(SELECT scope_id FROM inspection_runs WHERE id=$2::uuid) AND status='active' AND deleted_at IS NULL ON CONFLICT (idempotency_key) DO NOTHING`, finding.ID, runID, finding.Fingerprint)
		if err != nil {
			return mapError(err)
		}
	}
	return nil
}

func (s *store) ClaimDelivery(ctx context.Context) (Delivery, NotificationChannel, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Delivery{}, NotificationChannel{}, false, err
	}
	defer tx.Rollback(ctx)
	var d Delivery
	var c NotificationChannel
	err = tx.QueryRow(ctx, `WITH next AS (SELECT id FROM notification_deliveries WHERE status='queued' AND available_at<=now() ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE notification_deliveries d SET status='delivering',attempt=attempt+1,updated_at=now() FROM next JOIN notification_channels c ON c.id=d.channel_id WHERE d.id=next.id AND (SELECT count(*) FROM notification_deliveries recent WHERE recent.channel_id=c.id AND recent.created_at>=date_trunc('minute',now()) AND recent.status='succeeded') < c.rate_limit_per_minute RETURNING d.id::text,d.channel_id::text,COALESCE(d.finding_id::text,''),COALESCE(d.run_id::text,''),d.idempotency_key,d.status,d.attempt,COALESCE(d.response_status,0),d.response_body,d.error_message,c.id::text,c.scope_id::text,c.name,c.kind,c.webhook_url,c.credential_id::text,c.status,c.rate_limit_per_minute`).Scan(&d.ID, &d.ChannelID, &d.FindingID, &d.RunID, &d.IdempotencyKey, &d.Status, &d.Attempt, &d.ResponseStatus, &d.ResponseBody, &d.ErrorMessage, &c.ID, &c.ScopeID, &c.Name, &c.Kind, &c.WebhookURL, &c.CredentialID, &c.Status, &c.RateLimitPerMinute)
	if err == pgx.ErrNoRows {
		return Delivery{}, NotificationChannel{}, false, tx.Commit(ctx)
	}
	if err != nil {
		return Delivery{}, NotificationChannel{}, false, mapError(err)
	}
	return d, c, true, tx.Commit(ctx)
}
func (s *store) FinishDelivery(ctx context.Context, d Delivery, status int, body string, sendErr error) error {
	if sendErr == nil {
		_, err := s.pool.Exec(ctx, `UPDATE notification_deliveries SET status='succeeded',response_status=$2,response_body=$3,completed_at=now(),updated_at=now() WHERE id=$1::uuid`, d.ID, status, body)
		return mapError(err)
	}
	delay := time.Duration(1<<min(d.Attempt, 6)) * time.Second
	_, err := s.pool.Exec(ctx, `UPDATE notification_deliveries SET status='queued',response_status=$2,response_body=$3,error_message=$4,available_at=now()+$5::interval,updated_at=now() WHERE id=$1::uuid`, d.ID, status, body, errorText(sendErr), delay.String())
	return mapError(err)
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func (s *store) GetFinding(ctx context.Context, id string) (Finding, error) {
	var f Finding
	err := s.pool.QueryRow(ctx, `SELECT id::text,policy_id::text,target_resource_id::text,rule,identity_key,fingerprint,severity,message,status,first_observed_at,last_observed_at,resolved_at FROM inspection_findings WHERE id=$1::uuid`, id).Scan(&f.ID, &f.PolicyID, &f.TargetResourceID, &f.Rule, &f.IdentityKey, &f.Fingerprint, &f.Severity, &f.Message, &f.Status, &f.FirstObservedAt, &f.LastObservedAt, &f.ResolvedAt)
	if err == pgx.ErrNoRows {
		return Finding{}, ErrNotFound
	}
	return f, mapError(err)
}
func (s *store) SetPolicyStatus(ctx context.Context, id, scopeID, status string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE inspection_policies SET status=$3,updated_at=now() WHERE id=$1::uuid AND scope_id=$2::uuid AND deleted_at IS NULL`, id, scopeID, status)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}
func errorText(err error) string {
	if err == nil {
		return ""
	}
	v := err.Error()
	if len(v) > 1000 {
		return v[:1000]
	}
	return v
}
func mapError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("inspection store: %w", err)
}
