package inspection

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"opskeeper/backend/notification"
)

type notificationDeliveryQueue interface {
	ClaimNotificationDelivery(context.Context, string, time.Duration) (NotificationDeliveryJob, bool, error)
	FinishNotificationDelivery(context.Context, NotificationDeliveryJob, int, string, time.Duration, error) error
}

type claimedNotificationRoute struct {
	ID, RouteID, ScopeID, ChannelID, ChannelKind string
	ConfigCiphertext                             []byte
	KeyVersion                                   string
	Template                                     notificationTemplateRow
	EventType                                    string
	MaxBatchSize                                 int
	RateLimitPerMinute                           int
	Status                                       string
	AvailableAt                                  time.Time
	AggregationUntil                             time.Time
	Item                                         NotificationDeliveryItem
}

type notificationTemplateRow struct {
	Format, Title, Body string
	Payload, Variables  []byte
}

type NotificationQueueStats struct {
	Queued, Delivering, DeadLetter int64
	OldestQueued                   time.Duration
}

func (s *store) NotificationQueueStats(ctx context.Context) (NotificationQueueStats, error) {
	var stats NotificationQueueStats
	var oldestSeconds float64
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status='queued'),
		       count(*) FILTER (WHERE status='delivering'),
		       count(*) FILTER (WHERE status='dead_letter'),
	       COALESCE(extract(epoch FROM now()-min(created_at) FILTER (WHERE status='queued')),0)
		  FROM notification_deliveries`).Scan(&stats.Queued, &stats.Delivering, &stats.DeadLetter, &oldestSeconds)
	if err != nil {
		return NotificationQueueStats{}, mapError(err)
	}
	stats.OldestQueued = time.Duration(oldestSeconds * float64(time.Second))
	return stats, nil
}

func (s *store) ClaimNotificationDelivery(ctx context.Context, owner string, lease time.Duration) (NotificationDeliveryJob, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return NotificationDeliveryJob{}, false, err
	}
	defer tx.Rollback(ctx)
	var route claimedNotificationRoute
	var eventPayload []byte
	err = tx.QueryRow(ctx, `
		SELECT d.id::text,d.route_id::text,d.scope_id::text,d.channel_id::text,c.kind,
		       COALESCE(config.provider_config_ciphertext,''::bytea),COALESCE(config.key_version,''),
		       template.format,template.title_template,template.body_template,template.payload_template,template.variables,
	       event.event_type,event.payload,d.attempt,d.max_attempts,d.available_at,d.created_at,
	       event.id::text,COALESCE(d.route_snapshot->>'max_batch_size','1')::int,c.rate_limit_per_minute,d.status,
	       COALESCE((d.route_snapshot->>'aggregation_until')::timestamptz,d.available_at)
		  FROM notification_deliveries d
		  JOIN notification_events event ON event.id=d.event_id AND event.scope_id=d.scope_id
		  JOIN notification_channels c ON c.id=d.channel_id AND c.scope_id=d.rule_scope_id
		  JOIN notification_channel_versions config ON config.id=d.channel_version_id AND config.scope_id=d.rule_scope_id
		  JOIN notification_template_versions template ON template.id=d.template_version_id AND template.scope_id=d.rule_scope_id
		 WHERE ((d.status='queued' AND d.available_at<=now()) OR (d.status='delivering' AND d.lease_expires_at<=now()))
		   AND (SELECT count(*) FROM notification_delivery_attempts attempt
		         JOIN notification_deliveries recent ON recent.id=attempt.delivery_id
		        WHERE recent.channel_id=c.id AND attempt.started_at>=date_trunc('minute',now())) < c.rate_limit_per_minute
		 ORDER BY d.available_at,d.id
		 FOR UPDATE OF d,c SKIP LOCKED LIMIT 1`).
		Scan(&route.ID, &route.RouteID, &route.ScopeID, &route.ChannelID, &route.ChannelKind,
			&route.ConfigCiphertext, &route.KeyVersion,
			&route.Template.Format, &route.Template.Title, &route.Template.Body, &route.Template.Payload, &route.Template.Variables,
			&route.EventType, &eventPayload, &route.Item.Attempt, &route.Item.MaxAttempts, &route.AvailableAt, &route.Item.StartedAt,
			&route.Item.EventID, &route.MaxBatchSize, &route.RateLimitPerMinute, &route.Status, &route.AggregationUntil)
	if err == pgx.ErrNoRows {
		return NotificationDeliveryJob{}, false, tx.Commit(ctx)
	}
	if err != nil {
		return NotificationDeliveryJob{}, false, mapError(err)
	}
	if route.MaxBatchSize < 1 {
		route.MaxBatchSize = 1
	}
	if route.MaxBatchSize > 1000 {
		route.MaxBatchSize = 1000
	}
	if route.Status == "queued" {
		if tag, err := tx.Exec(ctx, `UPDATE notification_deliveries SET status='delivering',attempt=attempt+1,lease_owner=$2,lease_expires_at=now()+$3::interval,updated_at=now() WHERE id=$1::uuid`, route.ID, owner, lease.String()); err != nil || tag.RowsAffected() != 1 {
			return NotificationDeliveryJob{}, false, mapError(err)
		}
		route.Item.Attempt++
	} else {
		if _, err := tx.Exec(ctx, `UPDATE notification_deliveries SET attempt=attempt+1,lease_owner=$2,lease_expires_at=now()+$3::interval,updated_at=now() WHERE id=$1::uuid`, route.ID, owner, lease.String()); err != nil {
			return NotificationDeliveryJob{}, false, mapError(err)
		}
		route.Item.Attempt++
	}
	route.Item.ID = route.ID
	route.Item.EventType = route.EventType
	if err := json.Unmarshal(eventPayload, &route.Item.Payload); err != nil {
		return NotificationDeliveryJob{}, false, fmt.Errorf("decode notification event payload: %w", err)
	}
	items := []NotificationDeliveryItem{route.Item}
	if route.MaxBatchSize > 1 {
		rows, err := tx.Query(ctx, `
			SELECT d.id::text,event.id::text,event.event_type,event.payload,d.attempt,d.max_attempts,d.created_at
			  FROM notification_deliveries d
			  JOIN notification_events event ON event.id=d.event_id AND event.scope_id=d.scope_id
				 WHERE d.route_id=$1::uuid AND d.scope_id=$5::uuid AND d.status='queued' AND d.created_at<=$2
				   AND event.event_type=$3
				 ORDER BY d.created_at,d.id
				 FOR UPDATE OF d SKIP LOCKED LIMIT $4`, route.RouteID, route.AggregationUntil, route.EventType, route.MaxBatchSize-1, route.ScopeID)
		if err != nil {
			return NotificationDeliveryJob{}, false, mapError(err)
		}
		var siblings []NotificationDeliveryItem
		for rows.Next() {
			var item NotificationDeliveryItem
			var payload []byte
			if err := rows.Scan(&item.ID, &item.EventID, &item.EventType, &payload, &item.Attempt, &item.MaxAttempts, &item.StartedAt); err != nil {
				rows.Close()
				return NotificationDeliveryJob{}, false, err
			}
			if err := json.Unmarshal(payload, &item.Payload); err != nil {
				rows.Close()
				return NotificationDeliveryJob{}, false, err
			}
			siblings = append(siblings, item)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return NotificationDeliveryJob{}, false, err
		}
		rows.Close()
		for i := range siblings {
			item := &siblings[i]
			if _, err := tx.Exec(ctx, `UPDATE notification_deliveries SET status='delivering',attempt=attempt+1,lease_owner=$2,lease_expires_at=now()+$3::interval,updated_at=now() WHERE id=$1::uuid`, item.ID, owner, lease.String()); err != nil {
				return NotificationDeliveryJob{}, false, mapError(err)
			}
			item.Attempt++
		}
		items = append(items, siblings...)
	}
	if err := tx.Commit(ctx); err != nil {
		return NotificationDeliveryJob{}, false, err
	}
	var draft notification.TemplateDraft
	draft.Format, draft.TitleTemplate, draft.BodyTemplate = route.Template.Format, route.Template.Title, route.Template.Body
	if len(route.Template.Payload) > 0 {
		draft.PayloadTemplate = route.Template.Payload
	}
	if err := json.Unmarshal(route.Template.Variables, &draft.Variables); err != nil {
		return NotificationDeliveryJob{}, false, fmt.Errorf("decode notification template variables: %w", err)
	}
	return NotificationDeliveryJob{
		Owner: owner, ID: route.ID, ScopeID: route.ScopeID, ChannelID: route.ChannelID,
		ChannelKind: route.ChannelKind, ProviderConfigCiphertext: route.ConfigCiphertext,
		KeyVersion: route.KeyVersion, Template: draft, Items: items,
		RateLimitPerMinute: route.RateLimitPerMinute,
	}, true, nil
}

func (s *store) FinishNotificationDelivery(ctx context.Context, job NotificationDeliveryJob, status int, body string, retryAfter time.Duration, sendErr error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	response := notification.SanitizeSummary(body)
	if len(response) > 4000 {
		response = response[:4000]
	}
	for _, item := range job.Items {
		deliveryStatus, attemptStatus := "succeeded", "succeeded"
		delay := time.Duration(0)
		errorCode, errorMessage := "", ""
		completed := true
		if sendErr != nil || status < 200 || status >= 300 {
			attemptStatus = "retrying"
			deliveryStatus = "queued"
			completed = false
			delay = retryDelay(item.Attempt, retryAfter)
			errorCode = "provider"
			errorMessage = notification.SanitizeSummary(errorText(sendErr))
			if item.Attempt >= item.MaxAttempts {
				deliveryStatus, attemptStatus, completed = "dead_letter", "dead_letter", true
			}
		}
		if sendErr == nil && (status < 200 || status >= 300) {
			errorMessage = fmt.Sprintf("provider returned HTTP %d", status)
		}
		var tag pgconn.CommandTag
		if completed {
			tag, err = tx.Exec(ctx, `UPDATE notification_deliveries SET status=$3,response_status=NULLIF($4,0),response_body=$5,error_message=$6,lease_owner='',lease_expires_at=NULL,completed_at=now(),updated_at=now() WHERE id=$1::uuid AND lease_owner=$2 AND status='delivering'`, item.ID, job.Owner, deliveryStatus, status, response, errorMessage)
		} else {
			tag, err = tx.Exec(ctx, `UPDATE notification_deliveries SET status=$3,response_status=NULLIF($4,0),response_body=$5,error_message=$6,available_at=now()+$7::interval,lease_owner='',lease_expires_at=NULL,completed_at=NULL,updated_at=now() WHERE id=$1::uuid AND lease_owner=$2 AND status='delivering'`, item.ID, job.Owner, deliveryStatus, status, response, errorMessage, delay.String())
		}
		if err != nil {
			return mapError(err)
		}
		if tag.RowsAffected() != 1 {
			return ErrNotFound
		}
		if _, err := tx.Exec(ctx, `INSERT INTO notification_delivery_attempts(scope_id,delivery_id,attempt,status,response_status,response_body,error_code,error_message,started_at) SELECT scope_id,id,$2,$3,NULLIF($4,0),$5,$6,$7,now() FROM notification_deliveries WHERE id=$1::uuid`, item.ID, item.Attempt, attemptStatus, status, response, errorCode, errorMessage); err != nil {
			return mapError(err)
		}
	}
	return tx.Commit(ctx)
}

func retryDelay(attempt int, retryAfter time.Duration) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Duration(1<<min(attempt, 10)) * time.Second
	if retryAfter > delay {
		delay = retryAfter
	}
	if delay > time.Hour {
		return time.Hour
	}
	return delay
}

var _ notificationDeliveryQueue = (*store)(nil)
