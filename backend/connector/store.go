package connector

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CheckStore interface {
	Save(context.Context, Check) (Check, error)
	Latest(context.Context, string) (Check, error)
}

type store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) CheckStore { return &store{pool: pool} }

func (s *store) Save(ctx context.Context, input Check) (Check, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO resource_connection_checks
		       (resource_id, status, error_category, message, latency_ms, capabilities, checked_by, checked_at)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7::uuid, $8)
		RETURNING id::text`, input.ResourceID, input.Status, input.ErrorCategory, input.Message,
		input.LatencyMS, capabilitiesToStrings(input.Capabilities), input.CheckedBy, input.CheckedAt).Scan(&id)
	if err != nil {
		return Check{}, fmt.Errorf("save resource connection check: %w", err)
	}
	return s.get(ctx, id)
}

func (s *store) Latest(ctx context.Context, resourceID string) (Check, error) {
	return s.scan(s.pool.QueryRow(ctx, checkSelect+`
		WHERE check_record.resource_id = $1::uuid
		ORDER BY check_record.checked_at DESC, check_record.id DESC
		LIMIT 1`, resourceID))
}

func (s *store) get(ctx context.Context, id string) (Check, error) {
	return s.scan(s.pool.QueryRow(ctx, checkSelect+" WHERE check_record.id = $1::uuid", id))
}

func (s *store) scan(row pgx.Row) (Check, error) {
	var item Check
	var category string
	var capabilities []string
	if err := row.Scan(&item.ID, &item.ResourceID, &item.Status, &category, &item.Message,
		&item.LatencyMS, &capabilities, &item.CheckedBy, &item.CheckedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Check{}, ErrNotFound
		}
		return Check{}, fmt.Errorf("read resource connection check: %w", err)
	}
	item.ErrorCategory = Category(category)
	for _, capability := range capabilities {
		item.Capabilities = append(item.Capabilities, Capability(capability))
	}
	return item, nil
}

const checkSelect = `
	SELECT check_record.id::text, check_record.resource_id::text, check_record.status,
	       check_record.error_category, check_record.message, check_record.latency_ms,
	       check_record.capabilities, check_record.checked_by::text, check_record.checked_at
	  FROM resource_connection_checks check_record`

func capabilitiesToStrings(capabilities []Capability) []string {
	result := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		result = append(result, string(capability))
	}
	return result
}
