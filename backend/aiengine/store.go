package aiengine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type EventStore interface {
	AppendEvent(context.Context, Event) error
	ListEvents(context.Context, string, int64, int) ([]Event, error)
}

type ToolCallRecord struct {
	ExecutionID        string         `json:"execution_id"`
	Sequence           int            `json:"sequence"`
	ProviderResourceID string         `json:"provider_resource_id,omitempty"`
	ModelName          string         `json:"model_name,omitempty"`
	ResourceID         string         `json:"resource_id,omitempty"`
	ToolName           string         `json:"tool_name"`
	Arguments          map[string]any `json:"arguments,omitempty"`
	Output             any            `json:"output,omitempty"`
	Status             Status         `json:"status"`
	ErrorCode          string         `json:"error_code,omitempty"`
	Error              string         `json:"error,omitempty"`
	StartedAt          time.Time      `json:"started_at,omitempty"`
	CompletedAt        time.Time      `json:"completed_at,omitempty"`
	DurationMS         int64          `json:"duration_ms,omitempty"`
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
		INSERT INTO ai_execution_tool_calls (execution_id, sequence, provider_resource_id, model_name, resource_id, tool_name, arguments, output, status, error_code, error_message, started_at, completed_at, duration_ms)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, '')::uuid, $6, $7, $8, $9, NULLIF($10, ''), NULLIF($11, ''), COALESCE(NULLIF($12, '')::timestamptz, now()), NULLIF($13, '')::timestamptz, $14)
		ON CONFLICT (execution_id, sequence) DO UPDATE SET provider_resource_id = EXCLUDED.provider_resource_id, model_name = EXCLUDED.model_name, resource_id = EXCLUDED.resource_id, arguments = EXCLUDED.arguments, output = EXCLUDED.output, status = EXCLUDED.status, error_code = EXCLUDED.error_code, error_message = EXCLUDED.error_message, started_at = EXCLUDED.started_at, completed_at = EXCLUDED.completed_at, duration_ms = EXCLUDED.duration_ms`,
		record.ExecutionID, record.Sequence, record.ProviderResourceID, record.ModelName, record.ResourceID, record.ToolName, args, output, record.Status, record.ErrorCode, record.Error, nullableTime(record.StartedAt), nullableTime(record.CompletedAt), record.DurationMS)
	return err
}

func (s *PostgresStore) ListToolCalls(ctx context.Context, executionID string, limit int) ([]ToolCallRecord, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("AIEngine tool call store is unavailable")
	}
	if limit < 1 || limit > 500 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `SELECT execution_id, sequence, COALESCE(provider_resource_id::text, ''), COALESCE(model_name, ''), COALESCE(resource_id::text, ''), tool_name, arguments, output, status, COALESCE(error_code, ''), COALESCE(error_message, ''), started_at, completed_at, duration_ms FROM ai_execution_tool_calls WHERE execution_id = $1 ORDER BY sequence LIMIT $2`, executionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ToolCallRecord, 0)
	for rows.Next() {
		var item ToolCallRecord
		var arguments, output []byte
		if err := rows.Scan(&item.ExecutionID, &item.Sequence, &item.ProviderResourceID, &item.ModelName, &item.ResourceID, &item.ToolName, &arguments, &output, &item.Status, &item.ErrorCode, &item.Error, &item.StartedAt, &item.CompletedAt, &item.DurationMS); err != nil {
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

func nullableTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func redactValue(value any) any {
	switch item := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(item))
		for key, nested := range item {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "api_key") || strings.Contains(lower, "api-key") || strings.Contains(lower, "authorization") || lower == "key" {
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
