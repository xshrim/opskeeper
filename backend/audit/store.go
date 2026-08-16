package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type store struct {
	pool *pgxpool.Pool
}

var _ Store = (*store)(nil)

func NewStore(pool *pgxpool.Pool) Store {
	return &store{pool: pool}
}

func (s *store) Record(ctx context.Context, event Event) error {
	details, err := json.Marshal(event.Details)
	if err != nil {
		return fmt.Errorf("encode audit details: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO audit_events
			(actor_user_id, action, target_type, target_id, scope_id, result, request_id, client_ip, details, created_at)
		VALUES (NULLIF($1, '')::uuid, $2, $3, $4, NULLIF($5, '')::uuid, $6, $7, $8, $9, COALESCE(NULLIF($10::timestamptz, '0001-01-01 00:00:00+00'::timestamptz), now()))`,
		event.ActorUserID, event.Action, event.TargetType, event.TargetID, event.ScopeID,
		event.Result, event.RequestID, event.ClientIP, details, event.CreatedAt)
	if err != nil {
		return fmt.Errorf("record audit event: %w", err)
	}
	return nil
}

func (s *store) List(ctx context.Context, scopeIDs []string, limit int) (Page, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE scope_id = ANY($1::uuid[]) OR scope_id IS NULL`, scopeIDs).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("count audit events: %w", err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT COALESCE(actor_user_id::text, ''), action, target_type, target_id,
		       COALESCE(scope_id::text, ''), result, request_id, client_ip, details, created_at
		  FROM audit_events
		 WHERE scope_id = ANY($1::uuid[]) OR scope_id IS NULL
		 ORDER BY created_at DESC, id DESC LIMIT $2`, scopeIDs, limit)
	if err != nil {
		return Page{}, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()
	items := make([]Event, 0)
	for rows.Next() {
		var event Event
		var details []byte
		if err := rows.Scan(&event.ActorUserID, &event.Action, &event.TargetType, &event.TargetID, &event.ScopeID, &event.Result, &event.RequestID, &event.ClientIP, &details, &event.CreatedAt); err != nil {
			return Page{}, fmt.Errorf("scan audit event: %w", err)
		}
		if err := json.Unmarshal(details, &event.Details); err != nil {
			return Page{}, fmt.Errorf("decode audit event: %w", err)
		}
		items = append(items, event)
	}
	return Page{Items: items, Total: total}, rows.Err()
}
