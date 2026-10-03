package inspection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"opskeeper/backend/notification"
)

func reserveNotificationRuleCooldown(ctx context.Context, tx pgx.Tx, scopeID, ruleID string, event notificationEventInput, cooldown time.Duration) (bool, error) {
	if cooldown <= 0 || event.Type == notification.FindingResolved || event.Type == notification.FindingReopened {
		return true, nil
	}
	group := event.IdentityKey
	if group == "" {
		group = event.TargetResourceID
	}
	if group == "" {
		group = event.PolicyID
	}
	key := sha256.Sum256([]byte(string(event.Type) + ":" + group))
	var reserved string
	err := tx.QueryRow(ctx, `
		INSERT INTO notification_rule_cooldowns(scope_id,rule_id,cooldown_key,last_queued_at)
		VALUES($1::uuid,$2::uuid,$3,now())
		ON CONFLICT(rule_id,cooldown_key) DO UPDATE SET last_queued_at=excluded.last_queued_at
		 WHERE notification_rule_cooldowns.last_queued_at <= now()-make_interval(secs=>$4::double precision)
		RETURNING rule_id::text`, scopeID, ruleID, hex.EncodeToString(key[:]), cooldown.Seconds()).Scan(&reserved)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, mapError(err)
	}
	return true, nil
}

func (s *store) SetPolicyNotificationRules(ctx context.Context, scopeID, policyID string, ruleIDs []string) ([]string, error) {
	unique := make(map[string]bool, len(ruleIDs))
	ids := make([]string, 0, len(ruleIDs))
	for _, id := range ruleIDs {
		if id == "" {
			return nil, fmt.Errorf("notification rule ID is required")
		}
		if !unique[id] {
			unique[id] = true
			ids = append(ids, id)
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin policy notification rule update: %w", err)
	}
	defer tx.Rollback(ctx)
	var policyExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM inspection_policies WHERE id=$1::uuid AND scope_id=$2::uuid AND deleted_at IS NULL)`, policyID, scopeID).Scan(&policyExists); err != nil {
		return nil, mapError(err)
	}
	if !policyExists {
		return nil, ErrNotFound
	}
	var validCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM notification_rules WHERE scope_id=$1::uuid AND id=ANY($2::uuid[]) AND status='active' AND deleted_at IS NULL`, scopeID, ids).Scan(&validCount); err != nil {
		return nil, mapError(err)
	}
	if validCount != len(ids) {
		return nil, fmt.Errorf("notification rule is unavailable in the policy Scope")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM inspection_policy_notification_rules WHERE policy_id=$1::uuid AND scope_id=$2::uuid`, policyID, scopeID); err != nil {
		return nil, mapError(err)
	}
	if len(ids) > 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO inspection_policy_notification_rules(scope_id,policy_id,rule_id) SELECT $1::uuid,$2::uuid,rule_id FROM unnest($3::uuid[]) AS rule_id`, scopeID, policyID, ids); err != nil {
			return nil, mapError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit policy notification rules: %w", err)
	}
	return ids, nil
}

func (s *store) ListPolicyNotificationRules(ctx context.Context, scopeID, policyID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT policy_rule.rule_id::text FROM inspection_policy_notification_rules policy_rule JOIN inspection_policies policy ON policy.id=policy_rule.policy_id AND policy.scope_id=policy_rule.scope_id JOIN notification_rules rule ON rule.id=policy_rule.rule_id AND rule.scope_id=policy_rule.scope_id WHERE policy_rule.policy_id=$1::uuid AND policy_rule.scope_id=$2::uuid AND policy.deleted_at IS NULL AND rule.deleted_at IS NULL ORDER BY rule.name`, policyID, scopeID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}
