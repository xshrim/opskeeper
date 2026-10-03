package inspection

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"opskeeper/backend/authorization"
	"opskeeper/backend/notification"
)

type NotificationTemplateRecord struct {
	ID                string                         `json:"id"`
	ScopeID           string                         `json:"scope_id"`
	Name              string                         `json:"name"`
	ShareWithChildren bool                           `json:"share_with_children"`
	Inherited         bool                           `json:"inherited,omitempty"`
	Versions          []notification.TemplateVersion `json:"versions"`
}

type NotificationRoute struct {
	ChannelID         string `json:"channel_id"`
	TemplateVersionID string `json:"template_version_id"`
}

type NotificationRuleRecord struct {
	ID                   string `json:"id"`
	ScopeID              string `json:"scope_id"`
	ShareWithChildren    bool   `json:"share_with_children"`
	Inherited            bool   `json:"inherited,omitempty"`
	ShareWithChildrenSet bool   `json:"-"`
	notification.Rule
	Routes []NotificationRoute `json:"routes"`
}

type NotificationDeliveryRecord struct {
	ID          string                      `json:"id"`
	ScopeID     string                      `json:"scope_id"`
	EventType   string                      `json:"event_type"`
	Status      string                      `json:"status"`
	ChannelID   string                      `json:"channel_id"`
	RuleID      string                      `json:"rule_id"`
	Attempt     int                         `json:"attempt"`
	MaxAttempts int                         `json:"max_attempts"`
	AvailableAt time.Time                   `json:"available_at"`
	CreatedAt   time.Time                   `json:"created_at"`
	Payload     map[string]any              `json:"payload"`
	Attempts    []NotificationAttemptRecord `json:"attempts"`
}

type NotificationAttemptRecord struct {
	Attempt        int       `json:"attempt"`
	Status         string    `json:"status"`
	ResponseStatus int       `json:"response_status,omitempty"`
	ErrorCode      string    `json:"error_code,omitempty"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	StartedAt      time.Time `json:"started_at"`
	CompletedAt    time.Time `json:"completed_at"`
}

type notificationCatalogStore interface {
	CreateNotificationTemplate(context.Context, string, string, string, bool, notification.TemplateDraft) (NotificationTemplateRecord, error)
	ListNotificationTemplates(context.Context, string) ([]NotificationTemplateRecord, error)
	CreateNotificationTemplateVersion(context.Context, string, string, string, notification.TemplateDraft) (notification.TemplateVersion, error)
	GetNotificationTemplateVersion(context.Context, string, string, string) (notification.TemplateVersion, error)
	GetNotificationTemplateVersionByID(context.Context, string, string) (notification.TemplateVersion, error)
	PublishNotificationTemplate(context.Context, string, string, string) (notification.TemplateVersion, error)
	DeleteNotificationTemplate(context.Context, string, string) error
	SetNotificationTemplateSharing(context.Context, string, string, bool) error
	CreateNotificationRule(context.Context, string, string, NotificationRuleRecord) (NotificationRuleRecord, error)
	ListNotificationRules(context.Context, string) ([]NotificationRuleRecord, error)
	GetNotificationRuleRoutes(context.Context, string, string) ([]NotificationRoute, error)
	UpdateNotificationRule(context.Context, string, string, NotificationRuleRecord) (NotificationRuleRecord, error)
	DisableNotificationRule(context.Context, string, string) error
	SetPolicyNotificationRules(context.Context, string, string, []string) ([]string, error)
	ListPolicyNotificationRules(context.Context, string, string) ([]string, error)
	ListNotificationDeliveries(context.Context, string, string, string, int) ([]NotificationDeliveryRecord, error)
	RetryNotificationDelivery(context.Context, string, string) error
}

func (s *store) CreateNotificationTemplate(ctx context.Context, scopeID, name, actorID string, shareWithChildren bool, draft notification.TemplateDraft) (NotificationTemplateRecord, error) {
	template, _, err := notification.NewTemplateStore(s.pool).CreateWithSharing(ctx, scopeID, name, actorID, shareWithChildren, draft)
	if err != nil {
		return NotificationTemplateRecord{}, mapError(err)
	}
	return s.loadNotificationTemplate(ctx, scopeID, template.ID)
}

func (s *store) ListNotificationTemplates(ctx context.Context, scopeID string) ([]NotificationTemplateRecord, error) {
	rows, err := s.pool.Query(ctx, `SELECT template.id::text,template.scope_id::text,template.name,template.share_with_children FROM notification_templates template WHERE template.deleted_at IS NULL AND (template.scope_id=$1::uuid OR (template.share_with_children AND resource_scope_contains(template.scope_id,$1::uuid))) ORDER BY lower(template.name),template.id`, scopeID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]NotificationTemplateRecord, 0)
	for rows.Next() {
		var item NotificationTemplateRecord
		if err := rows.Scan(&item.ID, &item.ScopeID, &item.Name, &item.ShareWithChildren); err != nil {
			return nil, err
		}
		item.Inherited = item.ScopeID != scopeID
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range items {
		versions, err := notification.NewTemplateStore(s.pool).ListVersions(ctx, items[index].ID, items[index].ScopeID)
		if err != nil {
			return nil, err
		}
		items[index].Versions = versions
	}
	return items, nil
}

func (s *store) CreateNotificationTemplateVersion(ctx context.Context, templateID, scopeID, actorID string, draft notification.TemplateDraft) (notification.TemplateVersion, error) {
	item, err := notification.NewTemplateStore(s.pool).CreateVersion(ctx, templateID, scopeID, actorID, draft)
	if errors.Is(err, notification.ErrTemplateNotFound) {
		return notification.TemplateVersion{}, ErrNotFound
	}
	return item, mapError(err)
}

func (s *store) GetNotificationTemplateVersion(ctx context.Context, templateID, scopeID, versionID string) (notification.TemplateVersion, error) {
	item, err := scanNotificationTemplateVersion(s.pool.QueryRow(ctx, notificationTemplateVersionSelect+` WHERE version.id=$1::uuid AND version.template_id=$2::uuid AND template.deleted_at IS NULL AND (version.scope_id=$3::uuid OR (template.share_with_children AND resource_scope_contains(version.scope_id,$3::uuid)))`, versionID, templateID, scopeID))
	return item, err
}

func (s *store) GetNotificationTemplateVersionByID(ctx context.Context, scopeID, versionID string) (notification.TemplateVersion, error) {
	item, err := scanNotificationTemplateVersion(s.pool.QueryRow(ctx, notificationTemplateVersionSelect+` WHERE version.id=$1::uuid AND template.deleted_at IS NULL AND (version.scope_id=$2::uuid OR (template.share_with_children AND resource_scope_contains(version.scope_id,$2::uuid)))`, versionID, scopeID))
	return item, err
}

func (s *store) PublishNotificationTemplate(ctx context.Context, templateID, scopeID, versionID string) (notification.TemplateVersion, error) {
	item, err := notification.NewTemplateStore(s.pool).Publish(ctx, templateID, scopeID, versionID)
	if errors.Is(err, notification.ErrTemplateNotFound) {
		return notification.TemplateVersion{}, ErrNotFound
	}
	return item, mapError(err)
}

func (s *store) DeleteNotificationTemplate(ctx context.Context, scopeID, id string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE notification_templates SET deleted_at=now(),updated_at=now() WHERE id=$1::uuid AND scope_id=$2::uuid AND deleted_at IS NULL`, id, scopeID)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *store) SetNotificationTemplateSharing(ctx context.Context, scopeID, id string, share bool) error {
	tag, err := s.pool.Exec(ctx, `UPDATE notification_templates SET share_with_children=$3,updated_at=now() WHERE id=$1::uuid AND scope_id=$2::uuid AND deleted_at IS NULL`, id, scopeID, share)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *store) loadNotificationTemplate(ctx context.Context, scopeID, id string) (NotificationTemplateRecord, error) {
	var item NotificationTemplateRecord
	if err := s.pool.QueryRow(ctx, `SELECT id::text,name,share_with_children FROM notification_templates WHERE id=$1::uuid AND scope_id=$2::uuid AND deleted_at IS NULL`, id, scopeID).Scan(&item.ID, &item.Name, &item.ShareWithChildren); err != nil {
		return NotificationTemplateRecord{}, mapError(err)
	}
	item.ScopeID = scopeID
	versions, err := notification.NewTemplateStore(s.pool).ListVersions(ctx, id, scopeID)
	if err != nil {
		return NotificationTemplateRecord{}, err
	}
	item.Versions = versions
	return item, nil
}

const notificationTemplateVersionSelect = `SELECT version.id::text,version.template_id::text,version.scope_id::text,version.version,version.format,version.title_template,version.body_template,version.payload_template,version.variables,version.content_hash,version.status,version.created_at,version.published_at FROM notification_template_versions version JOIN notification_templates template ON template.id=version.template_id AND template.scope_id=version.scope_id`

func scanNotificationTemplateVersion(row pgx.Row) (notification.TemplateVersion, error) {
	var item notification.TemplateVersion
	var format, title, body string
	var payload, variables []byte
	if err := row.Scan(&item.ID, &item.TemplateID, &item.ScopeID, &item.Version, &format, &title, &body, &payload, &variables, &item.ContentHash, &item.Status, &item.CreatedAt, &item.PublishedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notification.TemplateVersion{}, ErrNotFound
		}
		return notification.TemplateVersion{}, mapError(err)
	}
	item.Draft = notification.TemplateDraft{Format: format, TitleTemplate: title, BodyTemplate: body}
	if len(payload) > 0 {
		item.Draft.PayloadTemplate = payload
	}
	if err := json.Unmarshal(variables, &item.Draft.Variables); err != nil {
		return notification.TemplateVersion{}, err
	}
	return item, nil
}

func (s *store) CreateNotificationRule(ctx context.Context, scopeID, actorID string, input NotificationRuleRecord) (NotificationRuleRecord, error) {
	validated, err := notification.ValidateRule(input.Rule)
	if err != nil {
		return NotificationRuleRecord{}, invalid(err.Error())
	}
	if validated.Status == "active" && len(input.Routes) == 0 {
		return NotificationRuleRecord{}, invalid("active notification rule requires at least one route")
	}
	filters, _ := json.Marshal(validated.Filters)
	silence, _ := json.Marshal(validated.Silence)
	events := make([]string, 0, len(validated.EventTypes))
	for _, event := range validated.EventTypes {
		events = append(events, string(event))
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return NotificationRuleRecord{}, err
	}
	defer tx.Rollback(ctx)
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO notification_rules(scope_id,name,status,share_with_children,event_types,minimum_severity,filters,cooldown_seconds,aggregation_seconds,max_batch_size,silence,created_by) VALUES($1::uuid,$2,$3,$4,$5::text[],$6,$7::jsonb,$8,$9,$10,$11::jsonb,NULLIF($12,'')::uuid) RETURNING id::text`, scopeID, validated.Name, validated.Status, input.ShareWithChildren, events, validated.MinimumSeverity, filters, int(validated.Cooldown.Seconds()), int(validated.Aggregation.Seconds()), validated.MaxBatchSize, silence, actorID).Scan(&id)
	if err != nil {
		return NotificationRuleRecord{}, mapError(err)
	}
	if err := insertNotificationRoutes(ctx, tx, scopeID, id, input.Routes); err != nil {
		return NotificationRuleRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return NotificationRuleRecord{}, err
	}
	return s.loadNotificationRule(ctx, scopeID, id)
}

func (s *store) ListNotificationRules(ctx context.Context, scopeID string) ([]NotificationRuleRecord, error) {
	rows, err := s.pool.Query(ctx, `SELECT rule.id::text,rule.scope_id::text,rule.share_with_children,rule.name,rule.status,rule.event_types,rule.minimum_severity,rule.filters,rule.cooldown_seconds,rule.aggregation_seconds,rule.max_batch_size,rule.silence FROM notification_rules rule WHERE rule.deleted_at IS NULL AND `+notificationRuleVisibleToScope+` ORDER BY lower(rule.name),rule.id`, scopeID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]NotificationRuleRecord, 0)
	for rows.Next() {
		item, err := scanNotificationRule(rows, scopeID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for index := range items {
		items[index].Routes, err = s.listNotificationRuleRoutes(ctx, items[index].ScopeID, items[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *store) GetNotificationRuleRoutes(ctx context.Context, scopeID, ruleID string) ([]NotificationRoute, error) {
	var ownerScope string
	if err := s.pool.QueryRow(ctx, `SELECT rule.scope_id::text FROM notification_rules rule WHERE rule.id=$2::uuid AND rule.status='active' AND rule.deleted_at IS NULL AND `+notificationRuleVisibleToScope, scopeID, ruleID).Scan(&ownerScope); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, mapError(err)
	}
	return s.listNotificationRuleRoutes(ctx, ownerScope, ruleID)
}

const notificationRuleVisibleToScope = `(rule.scope_id=$1::uuid OR (rule.share_with_children AND resource_scope_contains(rule.scope_id,$1::uuid) AND EXISTS (SELECT 1 FROM notification_rule_routes active_route WHERE active_route.rule_id=rule.id AND active_route.scope_id=rule.scope_id AND active_route.retired_at IS NULL) AND NOT EXISTS (SELECT 1 FROM notification_rule_routes route JOIN notification_channels channel ON channel.id=route.channel_id AND channel.scope_id=route.scope_id JOIN notification_template_versions version ON version.id=route.template_version_id AND version.scope_id=route.scope_id JOIN notification_templates template ON template.id=version.template_id AND template.scope_id=version.scope_id WHERE route.rule_id=rule.id AND route.scope_id=rule.scope_id AND route.retired_at IS NULL AND (NOT channel.share_with_children OR NOT template.share_with_children))))`

func (s *store) listNotificationRuleRoutes(ctx context.Context, scopeID, ruleID string) ([]NotificationRoute, error) {
	rows, err := s.pool.Query(ctx, `SELECT channel_id::text,template_version_id::text FROM notification_rule_routes WHERE rule_id=$1::uuid AND scope_id=$2::uuid AND retired_at IS NULL ORDER BY channel_id,template_version_id`, ruleID, scopeID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	routes := make([]NotificationRoute, 0)
	for rows.Next() {
		var route NotificationRoute
		if err := rows.Scan(&route.ChannelID, &route.TemplateVersionID); err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	return routes, rows.Err()
}

func (s *store) UpdateNotificationRule(ctx context.Context, scopeID, id string, input NotificationRuleRecord) (NotificationRuleRecord, error) {
	validated, err := notification.ValidateRule(input.Rule)
	if err != nil {
		return NotificationRuleRecord{}, invalid(err.Error())
	}
	if validated.Status == "active" && len(input.Routes) == 0 {
		return NotificationRuleRecord{}, invalid("active notification rule requires at least one route")
	}
	filters, _ := json.Marshal(validated.Filters)
	silence, _ := json.Marshal(validated.Silence)
	events := make([]string, 0, len(validated.EventTypes))
	for _, event := range validated.EventTypes {
		events = append(events, string(event))
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return NotificationRuleRecord{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE notification_rules SET name=$3,status=$4,share_with_children=CASE WHEN $12 THEN $13 ELSE share_with_children END,event_types=$5::text[],minimum_severity=$6,filters=$7::jsonb,cooldown_seconds=$8,aggregation_seconds=$9,max_batch_size=$10,silence=$11::jsonb,updated_at=now() WHERE id=$1::uuid AND scope_id=$2::uuid AND deleted_at IS NULL`, id, scopeID, validated.Name, validated.Status, events, validated.MinimumSeverity, filters, int(validated.Cooldown.Seconds()), int(validated.Aggregation.Seconds()), validated.MaxBatchSize, silence, input.ShareWithChildrenSet, input.ShareWithChildren)
	if err != nil {
		return NotificationRuleRecord{}, mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return NotificationRuleRecord{}, ErrNotFound
	}
	if _, err := tx.Exec(ctx, `UPDATE notification_rule_routes SET retired_at=now() WHERE rule_id=$1::uuid AND scope_id=$2::uuid AND retired_at IS NULL`, id, scopeID); err != nil {
		return NotificationRuleRecord{}, mapError(err)
	}
	if err := insertNotificationRoutes(ctx, tx, scopeID, id, input.Routes); err != nil {
		return NotificationRuleRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return NotificationRuleRecord{}, err
	}
	return s.loadNotificationRule(ctx, scopeID, id)
}

func (s *store) DisableNotificationRule(ctx context.Context, scopeID, id string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE notification_rules SET status='disabled',deleted_at=now(),updated_at=now() WHERE id=$1::uuid AND scope_id=$2::uuid AND deleted_at IS NULL`, id, scopeID)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *store) loadNotificationRule(ctx context.Context, scopeID, id string) (NotificationRuleRecord, error) {
	row := s.pool.QueryRow(ctx, `SELECT id::text,scope_id::text,share_with_children,name,status,event_types,minimum_severity,filters,cooldown_seconds,aggregation_seconds,max_batch_size,silence FROM notification_rules WHERE id=$1::uuid AND scope_id=$2::uuid AND deleted_at IS NULL`, id, scopeID)
	item, err := scanNotificationRule(row, scopeID)
	if err != nil {
		return NotificationRuleRecord{}, err
	}
	item.Routes, err = s.listNotificationRuleRoutes(ctx, scopeID, id)
	return item, err
}

type notificationRuleScanner interface{ Scan(...any) error }

func scanNotificationRule(row notificationRuleScanner, scopeID string) (NotificationRuleRecord, error) {
	var item NotificationRuleRecord
	var events []string
	var filtersJSON, silenceJSON []byte
	var cooldownSeconds, aggregationSeconds int
	if err := row.Scan(&item.ID, &item.ScopeID, &item.ShareWithChildren, &item.Name, &item.Status, &events, &item.MinimumSeverity, &filtersJSON, &cooldownSeconds, &aggregationSeconds, &item.MaxBatchSize, &silenceJSON); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return NotificationRuleRecord{}, ErrNotFound
		}
		return NotificationRuleRecord{}, mapError(err)
	}
	item.Inherited = item.ScopeID != scopeID
	item.Cooldown = time.Duration(cooldownSeconds) * time.Second
	item.Aggregation = time.Duration(aggregationSeconds) * time.Second
	for _, event := range events {
		item.EventTypes = append(item.EventTypes, notification.EventType(event))
	}
	if err := json.Unmarshal(filtersJSON, &item.Filters); err != nil {
		return NotificationRuleRecord{}, err
	}
	if err := json.Unmarshal(silenceJSON, &item.Silence); err != nil {
		return NotificationRuleRecord{}, err
	}
	return item, nil
}

func insertNotificationRoutes(ctx context.Context, tx pgx.Tx, scopeID, ruleID string, routes []NotificationRoute) error {
	seen := make(map[string]bool)
	for _, route := range routes {
		if route.ChannelID == "" || route.TemplateVersionID == "" {
			return invalid("notification routes require a channel and published template version")
		}
		key := route.ChannelID + ":" + route.TemplateVersionID
		if seen[key] {
			return invalid("duplicate notification route")
		}
		seen[key] = true
		var versionID string
		err := tx.QueryRow(ctx, `SELECT channel_version.id::text FROM notification_channels channel JOIN notification_channel_versions channel_version ON channel_version.channel_id=channel.id AND channel_version.scope_id=channel.scope_id AND channel_version.version=channel.config_version WHERE channel.id=$1::uuid AND channel.scope_id=$2::uuid AND channel.status='active' AND channel.deleted_at IS NULL`, route.ChannelID, scopeID).Scan(&versionID)
		if err != nil {
			return invalid("notification route channel is unavailable in this Scope")
		}
		var published bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM notification_template_versions WHERE id=$1::uuid AND scope_id=$2::uuid AND status='published')`, route.TemplateVersionID, scopeID).Scan(&published); err != nil || !published {
			return invalid("notification route template version is not published in this Scope")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO notification_rule_routes(scope_id,rule_id,channel_id,channel_version_id,template_version_id) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid)`, scopeID, ruleID, route.ChannelID, versionID, route.TemplateVersionID); err != nil {
			return mapError(err)
		}
	}
	return nil
}

func (s *store) ListNotificationDeliveries(ctx context.Context, scopeID, status, eventType string, limit int) ([]NotificationDeliveryRecord, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT delivery.id::text,delivery.scope_id::text,event.event_type,delivery.status,delivery.channel_id::text,delivery.rule_id::text,
		       delivery.attempt,delivery.max_attempts,delivery.available_at,delivery.created_at,event.payload
		  FROM notification_deliveries delivery JOIN notification_events event ON event.id=delivery.event_id AND event.scope_id=delivery.scope_id
		 WHERE delivery.scope_id=$1::uuid AND ($2='' OR delivery.status=$2) AND ($3='' OR event.event_type=$3)
		 ORDER BY delivery.created_at DESC,delivery.id LIMIT $4`, scopeID, status, eventType, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]NotificationDeliveryRecord, 0)
	for rows.Next() {
		var item NotificationDeliveryRecord
		var payload []byte
		if err := rows.Scan(&item.ID, &item.ScopeID, &item.EventType, &item.Status, &item.ChannelID, &item.RuleID, &item.Attempt, &item.MaxAttempts, &item.AvailableAt, &item.CreatedAt, &payload); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(payload, &item.Payload)
		attemptRows, err := s.pool.Query(ctx, `SELECT attempt,status,COALESCE(response_status,0),error_code,error_message,started_at,completed_at FROM notification_delivery_attempts WHERE delivery_id=$1::uuid ORDER BY attempt`, item.ID)
		if err != nil {
			return nil, mapError(err)
		}
		for attemptRows.Next() {
			var attempt NotificationAttemptRecord
			if err := attemptRows.Scan(&attempt.Attempt, &attempt.Status, &attempt.ResponseStatus, &attempt.ErrorCode, &attempt.ErrorMessage, &attempt.StartedAt, &attempt.CompletedAt); err != nil {
				attemptRows.Close()
				return nil, err
			}
			item.Attempts = append(item.Attempts, attempt)
		}
		attemptRows.Close()
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) RetryNotificationDelivery(ctx context.Context, scopeID, id string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE notification_deliveries SET status='queued',max_attempts=LEAST(max_attempts+5,20),available_at=now(),completed_at=NULL,error_message='',lease_owner='',lease_expires_at=NULL,updated_at=now() WHERE id=$1::uuid AND scope_id=$2::uuid AND status='dead_letter'`, id, scopeID)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = s.pool.Exec(ctx, `SELECT pg_notify('opskeeper_notification','')`)
	return mapError(err)
}

func (s *store) IsNotificationTemplatePublished(ctx context.Context, scopeID, templateVersionID string) (bool, error) {
	var published bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM notification_template_versions WHERE id=$1::uuid AND scope_id=$2::uuid AND status='published')`, templateVersionID, scopeID).Scan(&published)
	return published, mapError(err)
}

func validateNotificationScope(ctx context.Context, scopeID string) error {
	filter, restricted := authorization.ScopeFilterFromContext(ctx)
	if strings.TrimSpace(scopeID) == "" || !restricted || !filter.Allows(scopeID) {
		return authorization.ErrForbidden
	}
	return nil
}

var _ notificationCatalogStore = (*store)(nil)
