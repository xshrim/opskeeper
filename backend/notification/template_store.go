package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationTemplate struct {
	ID        string    `json:"id"`
	ScopeID   string    `json:"scope_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type TemplateVersion struct {
	ID          string        `json:"id"`
	TemplateID  string        `json:"template_id"`
	ScopeID     string        `json:"scope_id"`
	Version     int           `json:"version"`
	Draft       TemplateDraft `json:"draft"`
	ContentHash string        `json:"content_hash"`
	Status      string        `json:"status"`
	CreatedAt   time.Time     `json:"created_at"`
	PublishedAt *time.Time    `json:"published_at,omitempty"`
}

type TemplateStore struct{ pool *pgxpool.Pool }

func NewTemplateStore(pool *pgxpool.Pool) *TemplateStore { return &TemplateStore{pool: pool} }

func (s *TemplateStore) Create(ctx context.Context, scopeID, name, actorID string, draft TemplateDraft) (NotificationTemplate, TemplateVersion, error) {
	return s.CreateWithSharing(ctx, scopeID, name, actorID, false, draft)
}

func (s *TemplateStore) CreateWithSharing(ctx context.Context, scopeID, name, actorID string, shareWithChildren bool, draft TemplateDraft) (NotificationTemplate, TemplateVersion, error) {
	validated, hash, err := ValidateTemplate(draft)
	if err != nil {
		return NotificationTemplate{}, TemplateVersion{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return NotificationTemplate{}, TemplateVersion{}, fmt.Errorf("begin notification template: %w", err)
	}
	defer tx.Rollback(ctx)
	var item NotificationTemplate
	err = tx.QueryRow(ctx, `INSERT INTO notification_templates (scope_id, name, created_by, share_with_children) VALUES ($1::uuid, $2, NULLIF($3, '')::uuid, $4) RETURNING id::text, scope_id::text, name, created_at`, scopeID, name, actorID, shareWithChildren).
		Scan(&item.ID, &item.ScopeID, &item.Name, &item.CreatedAt)
	if err != nil {
		return NotificationTemplate{}, TemplateVersion{}, fmt.Errorf("create notification template: %w", err)
	}
	version, err := insertTemplateVersion(ctx, tx, item.ID, scopeID, actorID, validated, hash)
	if err != nil {
		return NotificationTemplate{}, TemplateVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return NotificationTemplate{}, TemplateVersion{}, fmt.Errorf("commit notification template: %w", err)
	}
	return item, version, nil
}

func (s *TemplateStore) CreateVersion(ctx context.Context, templateID, scopeID, actorID string, draft TemplateDraft) (TemplateVersion, error) {
	validated, hash, err := ValidateTemplate(draft)
	if err != nil {
		return TemplateVersion{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TemplateVersion{}, fmt.Errorf("begin notification template version: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, templateID); err != nil {
		return TemplateVersion{}, fmt.Errorf("lock notification template versions: %w", err)
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM notification_templates WHERE id = $1::uuid AND scope_id = $2::uuid AND deleted_at IS NULL)`, templateID, scopeID).Scan(&exists); err != nil {
		return TemplateVersion{}, err
	}
	if !exists {
		return TemplateVersion{}, ErrTemplateNotFound
	}
	version, err := insertTemplateVersion(ctx, tx, templateID, scopeID, actorID, validated, hash)
	if err != nil {
		return TemplateVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TemplateVersion{}, fmt.Errorf("commit notification template version: %w", err)
	}
	return version, nil
}

func (s *TemplateStore) Publish(ctx context.Context, templateID, scopeID, versionID string) (TemplateVersion, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TemplateVersion{}, fmt.Errorf("begin notification template publish: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, templateID); err != nil {
		return TemplateVersion{}, fmt.Errorf("lock notification template versions: %w", err)
	}
	tag, err := tx.Exec(ctx, `UPDATE notification_template_versions SET status = 'published', published_at = now() WHERE id = $1::uuid AND template_id = $2::uuid AND scope_id = $3::uuid AND status = 'draft'`, versionID, templateID, scopeID)
	if err != nil {
		return TemplateVersion{}, fmt.Errorf("publish notification template: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return TemplateVersion{}, ErrTemplateNotFound
	}
	version, err := getTemplateVersion(ctx, tx, versionID)
	if err != nil {
		return TemplateVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TemplateVersion{}, fmt.Errorf("commit notification template publish: %w", err)
	}
	return version, nil
}

func (s *TemplateStore) ListVersions(ctx context.Context, templateID, scopeID string) ([]TemplateVersion, error) {
	rows, err := s.pool.Query(ctx, templateVersionSelect+` WHERE template_id = $1::uuid AND scope_id = $2::uuid ORDER BY version DESC`, templateID, scopeID)
	if err != nil {
		return nil, fmt.Errorf("list notification template versions: %w", err)
	}
	defer rows.Close()
	items := make([]TemplateVersion, 0)
	for rows.Next() {
		item, err := scanTemplateVersion(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

var ErrTemplateNotFound = errors.New("notification template not found")

func insertTemplateVersion(ctx context.Context, tx pgx.Tx, templateID, scopeID, actorID string, draft TemplateDraft, hash string) (TemplateVersion, error) {
	variables, err := json.Marshal(draft.Variables)
	if err != nil {
		return TemplateVersion{}, err
	}
	payload := any(nil)
	if draft.Format == "json" {
		payload = draft.PayloadTemplate
	}
	var item TemplateVersion
	err = tx.QueryRow(ctx, `
		INSERT INTO notification_template_versions (scope_id, template_id, version, format, title_template, body_template, payload_template, variables, content_hash, created_by)
		SELECT $1::uuid, $2::uuid, COALESCE(max(version), 0) + 1, $4, $5, $6, $7::jsonb, $8::jsonb, $9, NULLIF($3, '')::uuid
		FROM notification_template_versions WHERE template_id = $2::uuid
		RETURNING id::text, template_id::text, scope_id::text, version, status, content_hash, created_at, published_at`, scopeID, templateID, actorID, draft.Format, draft.TitleTemplate, draft.BodyTemplate, payload, variables, hash).
		Scan(&item.ID, &item.TemplateID, &item.ScopeID, &item.Version, &item.Status, &item.ContentHash, &item.CreatedAt, &item.PublishedAt)
	if err != nil {
		return TemplateVersion{}, fmt.Errorf("create notification template version: %w", err)
	}
	item.Draft = draft
	return item, nil
}

const templateVersionSelect = `SELECT id::text, template_id::text, scope_id::text, version, format, title_template, body_template, payload_template, variables, content_hash, status, created_at, published_at FROM notification_template_versions`

type templateRowScanner interface{ Scan(...any) error }

func scanTemplateVersion(row templateRowScanner) (TemplateVersion, error) {
	var item TemplateVersion
	var format, title, body string
	var payload, variables []byte
	if err := row.Scan(&item.ID, &item.TemplateID, &item.ScopeID, &item.Version, &format, &title, &body, &payload, &variables, &item.ContentHash, &item.Status, &item.CreatedAt, &item.PublishedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TemplateVersion{}, ErrTemplateNotFound
		}
		return TemplateVersion{}, err
	}
	item.Draft = TemplateDraft{Format: format, TitleTemplate: title, BodyTemplate: body}
	if len(payload) != 0 {
		item.Draft.PayloadTemplate = payload
	}
	if err := json.Unmarshal(variables, &item.Draft.Variables); err != nil {
		return TemplateVersion{}, fmt.Errorf("decode notification template variables: %w", err)
	}
	return item, nil
}

func getTemplateVersion(ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, versionID string) (TemplateVersion, error) {
	return scanTemplateVersion(query.QueryRow(ctx, templateVersionSelect+` WHERE id = $1::uuid`, versionID))
}
