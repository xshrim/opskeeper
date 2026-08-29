package aiengine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type EventStore interface {
	AppendEvent(context.Context, Event) error
	ListEvents(context.Context, string, int64, int) ([]Event, error)
}

type ToolCallRecord struct {
	ExecutionID string
	Sequence    int
	ResourceID  string
	ToolName    string
	Arguments   map[string]any
	Output      any
	Status      Status
	Error       string
}

type ToolCallStore interface {
	RecordToolCall(context.Context, ToolCallRecord) error
	ListToolCalls(context.Context, string, int) ([]ToolCallRecord, error)
}

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) AppendEvent(ctx context.Context, event Event) error {
	if s == nil || s.pool == nil || strings.TrimSpace(event.ExecutionID) == "" {
		return fmt.Errorf("AIEngine event store is unavailable")
	}
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("encode AIEngine event payload: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO ai_execution_events (execution_id, sequence, type, status, occurred_at, payload)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (execution_id, sequence) DO UPDATE SET type = EXCLUDED.type, status = EXCLUDED.status, occurred_at = EXCLUDED.occurred_at, payload = EXCLUDED.payload`,
		event.ExecutionID, event.Sequence, event.Type, event.Status, event.Timestamp, payload)
	return err
}

func (s *PostgresStore) ListEvents(ctx context.Context, executionID string, after int64, limit int) ([]Event, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("AIEngine event store is unavailable")
	}
	if limit < 1 || limit > 500 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `SELECT execution_id, sequence, type, status, occurred_at, payload FROM ai_execution_events WHERE execution_id = $1 AND sequence > $2 ORDER BY sequence LIMIT $3`, executionID, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Event, 0)
	for rows.Next() {
		var item Event
		var payload []byte
		if err := rows.Scan(&item.ExecutionID, &item.Sequence, &item.Type, &item.Status, &item.Timestamp, &payload); err != nil {
			return nil, err
		}
		if len(payload) > 0 && string(payload) != "null" {
			if err := json.Unmarshal(payload, &item.Payload); err != nil {
				return nil, err
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) RecordToolCall(ctx context.Context, record ToolCallRecord) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("AIEngine tool call store is unavailable")
	}
	args, err := json.Marshal(redactValue(record.Arguments))
	if err != nil {
		return err
	}
	output, err := json.Marshal(redactValue(record.Output))
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO ai_execution_tool_calls (execution_id, sequence, resource_id, tool_name, arguments, output, status, error_message)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, NULLIF($8, ''))
		ON CONFLICT (execution_id, sequence) DO UPDATE SET arguments = EXCLUDED.arguments, output = EXCLUDED.output, status = EXCLUDED.status, error_message = EXCLUDED.error_message`,
		record.ExecutionID, record.Sequence, record.ResourceID, record.ToolName, args, output, record.Status, record.Error)
	return err
}

func (s *PostgresStore) ListToolCalls(ctx context.Context, executionID string, limit int) ([]ToolCallRecord, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("AIEngine tool call store is unavailable")
	}
	if limit < 1 || limit > 500 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `SELECT execution_id, sequence, COALESCE(resource_id::text, ''), tool_name, arguments, output, status, COALESCE(error_message, '') FROM ai_execution_tool_calls WHERE execution_id = $1 ORDER BY sequence LIMIT $2`, executionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ToolCallRecord, 0)
	for rows.Next() {
		var item ToolCallRecord
		var arguments, output []byte
		if err := rows.Scan(&item.ExecutionID, &item.Sequence, &item.ResourceID, &item.ToolName, &arguments, &output, &item.Status, &item.Error); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(arguments, &item.Arguments)
		if len(output) > 0 && string(output) != "null" {
			_ = json.Unmarshal(output, &item.Output)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func redactValue(value any) any {
	switch item := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(item))
		for key, nested := range item {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "api_key") || strings.Contains(lower, "authorization") {
				result[key] = "[REDACTED]"
				continue
			}
			result[key] = redactValue(nested)
		}
		return result
	case []any:
		result := make([]any, len(item))
		for index, nested := range item {
			result[index] = redactValue(nested)
		}
		return result
	default:
		return value
	}
}

var _ EventStore = (*PostgresStore)(nil)
var _ ToolCallStore = (*PostgresStore)(nil)
