package inspection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
	"opskeeper/backend/notification"
)

type Store interface {
	CreatePolicy(context.Context, Policy, string) (Policy, error)
	ListPolicies(context.Context, string) ([]Policy, error)
	ResolveTargets(context.Context, Policy) ([]string, error)
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
	RecordExplanation(context.Context, string, string) error
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
	err = tx.QueryRow(ctx, `INSERT INTO inspection_policies (scope_id,name,cron,timezone,status,target_labels,persona_id,timeout_seconds,retries,max_concurrent,max_tool_calls,max_tokens,maintenance_windows,created_by) VALUES ($1::uuid,$2,$3,$4,$5,$6,NULLIF($7,'')::uuid,$8,$9,$10,$11,$12,$13,NULLIF($14,'')::uuid) RETURNING id::text`, input.ScopeID, input.Name, input.Cron, input.Timezone, input.Status, labels, input.PersonaID, int(input.Timeout.Seconds()), input.Retries, input.MaxConcurrent, input.MaxToolCalls, input.MaxTokens, windows, actorID).Scan(&id)
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
	rows, err := s.pool.Query(ctx, `SELECT id::text,scope_id::text,name,cron,timezone,status,target_labels,COALESCE(persona_id::text,''),timeout_seconds,retries,max_concurrent,max_tool_calls,max_tokens,maintenance_windows FROM inspection_policies WHERE scope_id=$1::uuid AND deleted_at IS NULL ORDER BY created_at DESC`, scopeID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := []Policy{}
	for rows.Next() {
		var item Policy
		var labels, windows []byte
		var seconds int
		if err := rows.Scan(&item.ID, &item.ScopeID, &item.Name, &item.Cron, &item.Timezone, &item.Status, &labels, &item.PersonaID, &seconds, &item.Retries, &item.MaxConcurrent, &item.MaxToolCalls, &item.MaxTokens, &windows); err != nil {
			return nil, err
		}
		item.Timeout = time.Duration(seconds) * time.Second
		item.TimeoutSeconds = seconds
		_ = json.Unmarshal(labels, &item.TargetLabels)
		_ = json.Unmarshal(windows, &item.Maintenance)
		if err := s.pool.QueryRow(ctx, `SELECT COALESCE(array_agg(resource_id::text ORDER BY resource_id::text),'{}') FROM inspection_policy_targets WHERE policy_id=$1::uuid`, item.ID).Scan(&item.TargetResourceIDs); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ResolveTargets freezes the union of explicit targets and active resources
// matching the policy label selector. Scope and active-state checks happen in
// the database so a target disabled after policy creation is not inspected.
func (s *store) ResolveTargets(ctx context.Context, policy Policy) ([]string, error) {
	return resolveTargets(ctx, s.pool, policy)
}

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func resolveTargets(ctx context.Context, db queryer, policy Policy) ([]string, error) {
	if policy.TargetLabels == nil {
		policy.TargetLabels = map[string]string{}
	}
	labels, err := json.Marshal(policy.TargetLabels)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(ctx, `
		SELECT resource.id::text
		  FROM resources resource
		 WHERE resource.scope_id=$1::uuid
		   AND resource.status='active'
		   AND resource.deleted_at IS NULL
		   AND (($2::jsonb = '{}'::jsonb AND resource.id IN (
		          SELECT target.resource_id FROM inspection_policy_targets target WHERE target.policy_id=$3::uuid
		        )) OR ($2::jsonb <> '{}'::jsonb AND (
		          resource.labels @> $2::jsonb OR resource.id IN (
		            SELECT target.resource_id FROM inspection_policy_targets target WHERE target.policy_id=$3::uuid
		          )
		        )))
		 ORDER BY resource.id::text`, policy.ScopeID, string(labels), policy.ID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	targets := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		targets = append(targets, id)
	}
	return targets, rows.Err()
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
	rows, err := conn.Query(ctx, `SELECT id::text,scope_id::text,name,cron,timezone,status,target_labels,COALESCE(persona_id::text,''),timeout_seconds,retries,max_concurrent,max_tool_calls,max_tokens,maintenance_windows FROM inspection_policies WHERE status='active' AND deleted_at IS NULL`)
	if err != nil {
		return 0, mapError(err)
	}
	policies := []Policy{}
	for rows.Next() {
		policy, scanErr := scanPolicy(rows)
		if scanErr != nil {
			rows.Close()
			return 0, scanErr
		}
		policies = append(policies, policy)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	created := 0
	for _, policy := range policies {
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
		policy.TargetResourceIDs, err = resolveTargets(ctx, conn, policy)
		if err != nil {
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
	return created, nil
}

type policyScanner interface{ Scan(...any) error }

func scanPolicy(row policyScanner) (Policy, error) {
	var item Policy
	var labels, windows []byte
	var seconds int
	if err := row.Scan(&item.ID, &item.ScopeID, &item.Name, &item.Cron, &item.Timezone, &item.Status, &labels, &item.PersonaID, &seconds, &item.Retries, &item.MaxConcurrent, &item.MaxToolCalls, &item.MaxTokens, &windows); err != nil {
		return Policy{}, err
	}
	item.Timeout = time.Duration(seconds) * time.Second
	item.TimeoutSeconds = seconds
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
	if runErr != nil {
		var scopeID, policyID string
		if err := tx.QueryRow(ctx, `SELECT scope_id::text,policy_id::text FROM inspection_runs WHERE id=$1::uuid`, runID).Scan(&scopeID, &policyID); err != nil {
			return err
		}
		if err := s.recordNotificationEvent(ctx, tx, notificationEventInput{
			ScopeID: scopeID, PolicyID: policyID, RunID: runID,
			Type: notification.InspectionFailed, Key: "run:" + runID + ":inspection.failed",
			Payload: map[string]any{"event_type": notification.InspectionFailed, "policy_id": policyID, "run_id": runID, "error_code": "worker"},
		}); err != nil {
			return err
		}
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
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('inspection-results:'||$1,0))`, policy.ID); err != nil {
		return nil, nil, err
	}
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
		var previous *notification.FindingState
		var old Finding
		err := tx.QueryRow(ctx, `SELECT status,severity FROM inspection_findings WHERE policy_id=$1::uuid AND identity_key=$2 FOR UPDATE`, policy.ID, identity).Scan(&old.Status, &old.Severity)
		if err == nil {
			previous = &notification.FindingState{Status: old.Status, Severity: old.Severity}
		} else if err != pgx.ErrNoRows {
			return nil, nil, mapError(err)
		}
		var finding Finding
		if previous == nil {
			err = tx.QueryRow(ctx, `INSERT INTO inspection_findings (policy_id,scope_id,target_resource_id,rule,identity_key,fingerprint,severity,message,status,first_observed_at,last_observed_at,last_run_id,resolved_at) VALUES ($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,$7,$8,'open',now(),now(),$9::uuid,NULL) RETURNING id::text,policy_id::text,target_resource_id::text,rule,identity_key,fingerprint,severity,message,status,first_observed_at,last_observed_at,resolved_at`, policy.ID, policy.ScopeID, result.TargetResourceID, result.Rule, identity, fingerprint, result.Severity, result.Message, run.ID).Scan(&finding.ID, &finding.PolicyID, &finding.TargetResourceID, &finding.Rule, &finding.IdentityKey, &finding.Fingerprint, &finding.Severity, &finding.Message, &finding.Status, &finding.FirstObservedAt, &finding.LastObservedAt, &finding.ResolvedAt)
		} else {
			err = tx.QueryRow(ctx, `UPDATE inspection_findings SET fingerprint=$3,severity=$4,message=$5,status='open',last_observed_at=now(),last_run_id=$6::uuid,resolved_at=NULL WHERE policy_id=$1::uuid AND identity_key=$2 RETURNING id::text,policy_id::text,target_resource_id::text,rule,identity_key,fingerprint,severity,message,status,first_observed_at,last_observed_at,resolved_at`, policy.ID, identity, fingerprint, result.Severity, result.Message, run.ID).Scan(&finding.ID, &finding.PolicyID, &finding.TargetResourceID, &finding.Rule, &finding.IdentityKey, &finding.Fingerprint, &finding.Severity, &finding.Message, &finding.Status, &finding.FirstObservedAt, &finding.LastObservedAt, &finding.ResolvedAt)
		}
		if err != nil {
			return nil, nil, mapError(err)
		}
		findings = append(findings, finding)
		seen[identity] = true
		if eventType := notification.FindingTransition(previous, notification.FindingState{Status: finding.Status, Severity: finding.Severity}); eventType != "" {
			if err := s.recordFindingEvent(ctx, tx, run, policy, finding, eventType); err != nil {
				return nil, nil, err
			}
		}
	}
	// A successful deterministic observation that no longer returns a known
	// rule is a recovery. Scope it to frozen targets, never to a changing policy.
	for _, target := range policy.TargetResourceIDs {
		rows, err := tx.Query(ctx, `SELECT id::text,policy_id::text,target_resource_id::text,rule,identity_key,fingerprint,severity,message,status,first_observed_at,last_observed_at,resolved_at FROM inspection_findings WHERE policy_id=$1::uuid AND target_resource_id=$2::uuid AND status='open' AND identity_key <> ALL($3::text[]) FOR UPDATE`, policy.ID, target, keys(seen))
		if err != nil {
			return nil, nil, mapError(err)
		}
		resolved := []Finding{}
		for rows.Next() {
			var finding Finding
			if err := rows.Scan(&finding.ID, &finding.PolicyID, &finding.TargetResourceID, &finding.Rule, &finding.IdentityKey, &finding.Fingerprint, &finding.Severity, &finding.Message, &finding.Status, &finding.FirstObservedAt, &finding.LastObservedAt, &finding.ResolvedAt); err != nil {
				rows.Close()
				return nil, nil, err
			}
			resolved = append(resolved, finding)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, nil, err
		}
		rows.Close()
		for _, finding := range resolved {
			if _, err := tx.Exec(ctx, `UPDATE inspection_findings SET status='resolved',resolved_at=now(),last_observed_at=now(),last_run_id=$2::uuid WHERE id=$1::uuid`, finding.ID, run.ID); err != nil {
				return nil, nil, mapError(err)
			}
			if err := s.recordFindingEvent(ctx, tx, run, policy, finding, notification.FindingResolved); err != nil {
				return nil, nil, err
			}
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
	return findings, snapshots, nil
}

type notificationEventInput struct {
	ScopeID, PolicyID, RunID                           string
	Type                                               notification.EventType
	Key                                                string
	Payload                                            map[string]any
	FindingID, IdentityKey, TargetResourceID, Severity string
}

func (s *store) recordFindingEvent(ctx context.Context, tx pgx.Tx, run Run, policy Policy, finding Finding, eventType notification.EventType) error {
	key := fmt.Sprintf("run:%s:%s:%s", run.ID, eventType, finding.IdentityKey)
	return s.recordNotificationEvent(ctx, tx, notificationEventInput{
		ScopeID: policy.ScopeID, PolicyID: policy.ID, RunID: run.ID,
		Type: eventType, Key: key, FindingID: finding.ID, IdentityKey: finding.IdentityKey,
		TargetResourceID: finding.TargetResourceID, Severity: finding.Severity,
		Payload: map[string]any{
			"event_type": eventType, "policy_id": policy.ID, "run_id": run.ID,
			"finding_id": finding.ID, "finding_identity": finding.IdentityKey,
			"target_resource_id": finding.TargetResourceID, "rule": finding.Rule,
			"severity": finding.Severity, "finding_summary": notification.SanitizeSummary(finding.Message),
		},
	})
}

func (s *store) recordNotificationEvent(ctx context.Context, tx pgx.Tx, input notificationEventInput) error {
	payload, err := json.Marshal(input.Payload)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(payload)
	var eventID string
	err = tx.QueryRow(ctx, `INSERT INTO notification_events (scope_id,event_type,event_key,policy_id,run_id,finding_id,finding_identity,target_resource_id,severity,payload,content_hash) VALUES ($1::uuid,$2,$3,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,$7,NULLIF($8,'')::uuid,NULLIF($9,''),$10::jsonb,$11) ON CONFLICT (event_key) DO NOTHING RETURNING id::text`, input.ScopeID, input.Type, input.Key, input.PolicyID, input.RunID, input.FindingID, input.IdentityKey, input.TargetResourceID, input.Severity, string(payload), hex.EncodeToString(hash[:])).Scan(&eventID)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return mapError(err)
	}
	return s.enqueueEventDeliveries(ctx, tx, eventID, input)
}

func (s *store) enqueueEventDeliveries(ctx context.Context, tx pgx.Tx, eventID string, event notificationEventInput) error {
	rows, err := tx.Query(ctx, `
		SELECT rule.id::text, route.id::text, channel.id::text, config.id::text,
		       config.version, config.provider_config_hash, template.id::text,
		       template.version, template.content_hash, rule.event_types, rule.minimum_severity,
		       rule.filters, rule.cooldown_seconds, rule.aggregation_seconds, rule.max_batch_size,
		       rule.silence, COALESCE(event.severity,''), COALESCE(event.target_resource_id::text,''),
		       COALESCE(event.payload->>'rule','')
		  FROM notification_events event
	  JOIN inspection_policy_notification_rules policy_rule
		    ON policy_rule.policy_id=event.policy_id AND policy_rule.scope_id=event.scope_id
		  JOIN notification_rules rule
		    ON rule.id=policy_rule.rule_id AND rule.scope_id=event.scope_id
		  JOIN notification_rule_routes route
		    ON route.rule_id=rule.id AND route.scope_id=event.scope_id
		  JOIN notification_channels channel
		    ON channel.id=route.channel_id AND channel.scope_id=event.scope_id
		  JOIN notification_channel_versions config
		    ON config.id=route.channel_version_id AND config.scope_id=event.scope_id
		  JOIN notification_template_versions template
		    ON template.id=route.template_version_id AND template.scope_id=event.scope_id
		 WHERE event.id=$1::uuid AND rule.status='active' AND rule.deleted_at IS NULL
		   AND channel.status='active' AND channel.deleted_at IS NULL
		   AND template.status='published'
		 ORDER BY rule.id, route.id`, eventID)
	if err != nil {
		return mapError(err)
	}
	type route struct {
		RuleID, RouteID, ChannelID, ChannelVersionID string
		ChannelVersion                               int
		ChannelConfigHash                            string
		TemplateVersionID                            string
		TemplateVersion                              int
		TemplateHash                                 string
		Rule                                         notification.Rule
		RuleEvent                                    notification.RuleEvent
		Silence                                      notification.SilenceWindow
		AvailableAt                                  time.Time
	}
	routes := []route{}
	for rows.Next() {
		var item route
		var eventTypes []string
		var filtersJSON, silenceJSON []byte
		var minimumSeverity string
		var cooldownSeconds, aggregationSeconds, maxBatchSize int
		if err := rows.Scan(&item.RuleID, &item.RouteID, &item.ChannelID, &item.ChannelVersionID, &item.ChannelVersion, &item.ChannelConfigHash, &item.TemplateVersionID, &item.TemplateVersion, &item.TemplateHash, &eventTypes, &minimumSeverity, &filtersJSON, &cooldownSeconds, &aggregationSeconds, &maxBatchSize, &silenceJSON, &item.RuleEvent.Severity, &item.RuleEvent.ResourceID, &item.RuleEvent.FindingRule); err != nil {
			rows.Close()
			return err
		}
		item.RuleEvent.Type = event.Type
		item.RuleEvent.FindingIdentity = event.IdentityKey
		item.Rule = notification.Rule{Name: item.RuleID, Status: "active", MinimumSeverity: minimumSeverity, Cooldown: time.Duration(cooldownSeconds) * time.Second, Aggregation: time.Duration(aggregationSeconds) * time.Second, MaxBatchSize: maxBatchSize}
		for _, eventType := range eventTypes {
			item.Rule.EventTypes = append(item.Rule.EventTypes, notification.EventType(eventType))
		}
		filters, err := notification.DecodeRuleFilters(filtersJSON)
		if err != nil {
			rows.Close()
			return err
		}
		item.Rule.Filters = filters
		if err := json.Unmarshal(silenceJSON, &item.Silence); err != nil {
			rows.Close()
			return fmt.Errorf("decode notification silence window: %w", err)
		}
		if !notification.RuleMatches(item.Rule, item.RuleEvent) {
			continue
		}
		routes = append(routes, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	allowedByRule := make(map[string]bool)
	checkedRule := make(map[string]bool)
	for i := range routes {
		route := &routes[i]
		if !checkedRule[route.RuleID] {
			allowed, err := reserveNotificationRuleCooldown(ctx, tx, event.ScopeID, route.RuleID, event, route.Rule.Cooldown)
			if err != nil {
				return err
			}
			allowedByRule[route.RuleID] = allowed
			checkedRule[route.RuleID] = true
		}
		if !allowedByRule[route.RuleID] {
			continue
		}
		now := time.Now().UTC()
		allowedAt, err := notification.NextAllowedAt(route.Silence, now)
		if err != nil {
			return err
		}
		aggregateUntil := now.Add(route.Rule.Aggregation)
		if aggregateUntil.After(allowedAt) {
			allowedAt = aggregateUntil
		}
		route.AvailableAt = allowedAt
	}
	for _, route := range routes {
		if !allowedByRule[route.RuleID] {
			continue
		}
		snapshot, err := json.Marshal(map[string]any{
			"scope_id": event.ScopeID, "event_id": eventID, "policy_id": event.PolicyID, "event_type": event.Type,
			"rule_id": route.RuleID, "route_id": route.RouteID,
			"channel_id": route.ChannelID, "channel_version_id": route.ChannelVersionID,
			"channel_version": route.ChannelVersion, "channel_config_hash": route.ChannelConfigHash,
			"template_version_id": route.TemplateVersionID, "template_version": route.TemplateVersion,
			"template_hash": route.TemplateHash, "max_batch_size": route.Rule.MaxBatchSize,
			"aggregation_seconds": int(route.Rule.Aggregation.Seconds()), "available_at": route.AvailableAt,
		})
		if err != nil {
			return err
		}
		snapshotHash := sha256.Sum256(snapshot)
		idempotencyKey := eventID + ":" + route.RouteID
		result, err := tx.Exec(ctx, `INSERT INTO notification_deliveries (scope_id,event_id,policy_id,rule_id,route_id,channel_id,channel_version_id,template_version_id,finding_id,run_id,idempotency_key,route_snapshot,snapshot_hash,available_at) VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,$6::uuid,$7::uuid,$8::uuid,NULLIF($9,'')::uuid,NULLIF($10,'')::uuid,$11,$12::jsonb,$13,$14) ON CONFLICT (idempotency_key) DO NOTHING`, event.ScopeID, eventID, event.PolicyID, route.RuleID, route.RouteID, route.ChannelID, route.ChannelVersionID, route.TemplateVersionID, event.FindingID, event.RunID, idempotencyKey, string(snapshot), hex.EncodeToString(snapshotHash[:]), route.AvailableAt)
		if err != nil {
			return mapError(err)
		}
		if result.RowsAffected() == 1 {
			if _, err := tx.Exec(ctx, `SELECT pg_notify('opskeeper_notification','')`); err != nil {
				return err
			}
		}
	}
	return nil
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
	err := s.pool.QueryRow(ctx, `INSERT INTO notification_channels(scope_id,name,kind,webhook_url,status,rate_limit_per_minute) VALUES ($1::uuid,$2,'webhook',$3,$4,$5) RETURNING id::text`, item.ScopeID, item.Name, item.WebhookURL, item.Status, item.RateLimitPerMinute).Scan(&item.ID)
	if err != nil {
		return NotificationChannel{}, mapError(err)
	}
	item.Kind = "webhook"
	return item, nil
}

func (s *store) CreateConfiguredChannel(ctx context.Context, item NotificationChannel, ciphertext []byte, keyVersion, configHash string) (NotificationChannel, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return NotificationChannel{}, err
	}
	defer tx.Rollback(ctx)
	if err := tx.QueryRow(ctx, `INSERT INTO notification_channels(scope_id,name,kind,webhook_url,status,rate_limit_per_minute,config_version) VALUES($1::uuid,$2,$3,'https://encrypted.invalid/',$4,$5,1) RETURNING id::text`, item.ScopeID, item.Name, item.Kind, item.Status, item.RateLimitPerMinute).Scan(&item.ID); err != nil {
		return NotificationChannel{}, mapError(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO notification_channel_versions(scope_id,channel_id,version,provider_config_ciphertext,provider_config_hash,key_version) VALUES($1::uuid,$2::uuid,1,$3,$4,$5)`, item.ScopeID, item.ID, ciphertext, configHash, keyVersion); err != nil {
		return NotificationChannel{}, mapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return NotificationChannel{}, err
	}
	item.ConfigVersion = 1
	return item, nil
}

func (s *store) ListConfiguredChannels(ctx context.Context, scopeID string) ([]encryptedChannel, error) {
	rows, err := s.pool.Query(ctx, `SELECT channel.id::text,channel.scope_id::text,channel.name,channel.kind,channel.status,channel.rate_limit_per_minute,channel.config_version,channel.webhook_url,COALESCE(version.provider_config_ciphertext,''::bytea),COALESCE(version.key_version,'') FROM notification_channels channel LEFT JOIN notification_channel_versions version ON version.channel_id=channel.id AND version.version=channel.config_version WHERE channel.scope_id=$1::uuid AND channel.deleted_at IS NULL ORDER BY channel.created_at DESC`, scopeID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := []encryptedChannel{}
	for rows.Next() {
		var item encryptedChannel
		if err := rows.Scan(&item.Channel.ID, &item.Channel.ScopeID, &item.Channel.Name, &item.Channel.Kind, &item.Channel.Status, &item.Channel.RateLimitPerMinute, &item.Channel.ConfigVersion, &item.Channel.WebhookURL, &item.Ciphertext, &item.KeyVersion); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) GetConfiguredChannel(ctx context.Context, id, scopeID string) (encryptedChannel, error) {
	var item encryptedChannel
	err := s.pool.QueryRow(ctx, `SELECT channel.id::text,channel.scope_id::text,channel.name,channel.kind,channel.status,channel.rate_limit_per_minute,channel.config_version,channel.webhook_url,COALESCE(version.provider_config_ciphertext,''::bytea),COALESCE(version.key_version,'') FROM notification_channels channel LEFT JOIN notification_channel_versions version ON version.channel_id=channel.id AND version.version=channel.config_version WHERE channel.id=$1::uuid AND channel.scope_id=$2::uuid AND channel.deleted_at IS NULL`, id, scopeID).Scan(&item.Channel.ID, &item.Channel.ScopeID, &item.Channel.Name, &item.Channel.Kind, &item.Channel.Status, &item.Channel.RateLimitPerMinute, &item.Channel.ConfigVersion, &item.Channel.WebhookURL, &item.Ciphertext, &item.KeyVersion)
	if err == pgx.ErrNoRows {
		return encryptedChannel{}, ErrNotFound
	}
	return item, mapError(err)
}

func (s *store) UpdateConfiguredChannel(ctx context.Context, item NotificationChannel, ciphertext []byte, keyVersion, configHash string) (NotificationChannel, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return NotificationChannel{}, err
	}
	defer tx.Rollback(ctx)
	var nextVersion int
	err = tx.QueryRow(ctx, `UPDATE notification_channels SET name=$3,status=$4,rate_limit_per_minute=$5,config_version=config_version+1,updated_at=now() WHERE id=$1::uuid AND scope_id=$2::uuid AND deleted_at IS NULL RETURNING config_version`, item.ID, item.ScopeID, item.Name, item.Status, item.RateLimitPerMinute).Scan(&nextVersion)
	if err == pgx.ErrNoRows {
		return NotificationChannel{}, ErrNotFound
	}
	if err != nil {
		return NotificationChannel{}, mapError(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO notification_channel_versions(scope_id,channel_id,version,provider_config_ciphertext,provider_config_hash,key_version) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6)`, item.ScopeID, item.ID, nextVersion, ciphertext, configHash, keyVersion); err != nil {
		return NotificationChannel{}, mapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return NotificationChannel{}, err
	}
	item.ConfigVersion = nextVersion
	return item, nil
}

func (s *store) DeleteConfiguredChannel(ctx context.Context, id, scopeID string) error {
	result, err := s.pool.Exec(ctx, `UPDATE notification_channels SET status='disabled',deleted_at=now(),updated_at=now() WHERE id=$1::uuid AND scope_id=$2::uuid AND deleted_at IS NULL`, id, scopeID)
	if err != nil {
		return mapError(err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *store) ReserveChannelTest(ctx context.Context, id string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var limit int
	var scopeID string
	if err := tx.QueryRow(ctx, `SELECT rate_limit_per_minute,scope_id::text FROM notification_channels WHERE id=$1::uuid AND status='active' AND deleted_at IS NULL FOR UPDATE`, id).Scan(&limit, &scopeID); err == pgx.ErrNoRows {
		return ErrNotFound
	} else if err != nil {
		return mapError(err)
	}
	var count int
	err = tx.QueryRow(ctx, `INSERT INTO notification_channel_test_limits(channel_id,scope_id,window_started_at,attempts) VALUES($1::uuid,$2::uuid,date_trunc('minute',now()),1) ON CONFLICT(channel_id) DO UPDATE SET window_started_at=CASE WHEN notification_channel_test_limits.window_started_at < now()-interval '1 minute' THEN date_trunc('minute',now()) ELSE notification_channel_test_limits.window_started_at END,attempts=CASE WHEN notification_channel_test_limits.window_started_at < now()-interval '1 minute' THEN 1 ELSE notification_channel_test_limits.attempts+1 END RETURNING attempts`, id, scopeID).Scan(&count)
	if err != nil {
		return mapError(err)
	}
	if count > limit {
		return ErrConflict
	}
	return tx.Commit(ctx)
}

func (s *store) ListChannels(ctx context.Context, scopeID string) ([]NotificationChannel, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,scope_id::text,name,kind,webhook_url,status,rate_limit_per_minute FROM notification_channels WHERE scope_id=$1::uuid AND deleted_at IS NULL ORDER BY created_at DESC`, scopeID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := []NotificationChannel{}
	for rows.Next() {
		var n NotificationChannel
		if err := rows.Scan(&n.ID, &n.ScopeID, &n.Name, &n.Kind, &n.WebhookURL, &n.Status, &n.RateLimitPerMinute); err != nil {
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
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var scopeID, policyID string
	var deterministicCompleted bool
	if err := tx.QueryRow(ctx, `SELECT scope_id::text,policy_id::text,deterministic_completed FROM inspection_runs WHERE id=$1::uuid FOR UPDATE`, runID).Scan(&scopeID, &policyID, &deterministicCompleted); err != nil {
		return mapError(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE inspection_runs SET llm_status=$2,updated_at=now() WHERE id=$1::uuid`, runID, status); err != nil {
		return mapError(err)
	}
	if deterministicCompleted && (status == "degraded" || status == "failed") {
		if err := s.recordNotificationEvent(ctx, tx, notificationEventInput{
			ScopeID: scopeID, PolicyID: policyID, RunID: runID,
			Type: notification.InspectionDegraded, Key: "run:" + runID + ":inspection.degraded",
			Payload: map[string]any{"event_type": notification.InspectionDegraded, "policy_id": policyID, "run_id": runID, "llm_status": status},
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *store) RecordExplanation(ctx context.Context, runID, detail string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO inspection_run_steps(run_id,sequence,kind,status,detail,started_at,completed_at)
		VALUES ($1::uuid,COALESCE((SELECT max(sequence)+1 FROM inspection_run_steps WHERE run_id=$1::uuid),1),'ai_explanation','succeeded',$2,now(),now())`, runID, detail)
	return mapError(err)
}

func (s *store) ClaimDelivery(ctx context.Context) (Delivery, NotificationChannel, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Delivery{}, NotificationChannel{}, false, err
	}
	defer tx.Rollback(ctx)
	var d Delivery
	var c NotificationChannel
	err = tx.QueryRow(ctx, `WITH next AS (SELECT delivery.id,delivery.channel_id,COALESCE(event.event_type,'') AS event_type,event.payload AS event_payload FROM notification_deliveries delivery LEFT JOIN notification_events event ON event.id=delivery.event_id WHERE delivery.status='queued' AND delivery.available_at<=now() ORDER BY delivery.available_at,delivery.id FOR UPDATE OF delivery SKIP LOCKED LIMIT 1) UPDATE notification_deliveries d SET status='delivering',attempt=attempt+1,updated_at=now(),lease_owner='legacy-worker',lease_expires_at=now()+interval '30 seconds' FROM next JOIN notification_channels c ON c.id=next.channel_id WHERE d.id=next.id AND (SELECT count(*) FROM notification_deliveries recent WHERE recent.channel_id=c.id AND recent.created_at>=date_trunc('minute',now()) AND recent.status='succeeded') < c.rate_limit_per_minute RETURNING d.id::text,d.scope_id::text,d.channel_id::text,COALESCE(d.finding_id::text,''),COALESCE(d.run_id::text,''),next.event_type,next.event_payload,d.idempotency_key,d.status,d.attempt,d.max_attempts,COALESCE(d.response_status,0),d.response_body,d.error_message,now(),c.id::text,c.scope_id::text,c.name,c.kind,c.webhook_url,c.status,c.rate_limit_per_minute`).Scan(&d.ID, &d.ScopeID, &d.ChannelID, &d.FindingID, &d.RunID, &d.EventType, &d.EventPayload, &d.IdempotencyKey, &d.Status, &d.Attempt, &d.MaxAttempts, &d.ResponseStatus, &d.ResponseBody, &d.ErrorMessage, &d.StartedAt, &c.ID, &c.ScopeID, &c.Name, &c.Kind, &c.WebhookURL, &c.Status, &c.RateLimitPerMinute)
	if err == pgx.ErrNoRows {
		return Delivery{}, NotificationChannel{}, false, tx.Commit(ctx)
	}
	if err != nil {
		return Delivery{}, NotificationChannel{}, false, mapError(err)
	}
	return d, c, true, tx.Commit(ctx)
}
func (s *store) FinishDelivery(ctx context.Context, d Delivery, status int, body string, sendErr error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if d.StartedAt.IsZero() {
		d.StartedAt = time.Now().UTC()
	}
	if sendErr == nil {
		if _, err := tx.Exec(ctx, `UPDATE notification_deliveries SET status='succeeded',response_status=$2,response_body=$3,completed_at=now(),lease_owner='',lease_expires_at=NULL,updated_at=now() WHERE id=$1::uuid`, d.ID, status, body); err != nil {
			return mapError(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO notification_delivery_attempts (scope_id,delivery_id,attempt,status,response_status,response_body,started_at) VALUES ($1::uuid,$2::uuid,$3,'succeeded',$4,$5,$6)`, d.ScopeID, d.ID, d.Attempt, status, body, d.StartedAt); err != nil {
			return mapError(err)
		}
		return tx.Commit(ctx)
	}
	delay := time.Duration(1<<min(d.Attempt, 6)) * time.Second
	deliveryStatus, attemptStatus := "queued", "retrying"
	maxAttempts := d.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 5
	}
	if d.Attempt >= maxAttempts {
		deliveryStatus, attemptStatus = "dead_letter", "dead_letter"
	}
	if _, err := tx.Exec(ctx, `UPDATE notification_deliveries SET status=$2,response_status=$3,response_body=$4,error_message=$5,available_at=now()+$6::interval,lease_owner='',lease_expires_at=NULL,completed_at=CASE WHEN $2='dead_letter' THEN now() ELSE NULL END,updated_at=now() WHERE id=$1::uuid`, d.ID, deliveryStatus, status, body, errorText(sendErr), delay.String()); err != nil {
		return mapError(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO notification_delivery_attempts (scope_id,delivery_id,attempt,status,response_status,response_body,error_code,error_message,started_at) VALUES ($1::uuid,$2::uuid,$3,$4,$5,$6,'delivery_failed',$7,$8)`, d.ScopeID, d.ID, d.Attempt, attemptStatus, status, body, errorText(sendErr), d.StartedAt); err != nil {
		return mapError(err)
	}
	return tx.Commit(ctx)
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
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return fmt.Errorf("inspection store: %w", ErrConflict)
	}
	return fmt.Errorf("inspection store: %w", err)
}
