package inspection

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"opskeeper/backend/notification"
	"opskeeper/backend/secret"
)

type NotificationWorker struct {
	Store         Store
	Sender        NotificationTester
	Cipher        secret.Decryptor
	Owner         string
	LeaseDuration time.Duration
}

func (w NotificationWorker) RunOnce(ctx context.Context) (bool, error) {
	queue, ok := w.Store.(notificationDeliveryQueue)
	if !ok {
		return false, errors.New("notification store does not implement the PostgreSQL delivery queue")
	}
	owner := strings.TrimSpace(w.Owner)
	if owner == "" {
		owner = "notification-worker"
	}
	lease := w.LeaseDuration
	if lease <= 0 {
		lease = 45 * time.Second
	}
	job, claimed, err := queue.ClaimNotificationDelivery(ctx, owner, lease)
	if err != nil || !claimed {
		return claimed, err
	}
	var status int
	var body string
	var retryAfter time.Duration
	if w.Cipher == nil {
		err = errors.New("notification credential decryptor is unavailable")
	} else {
		var clear []byte
		clear, err = w.Cipher.Decrypt(job.ProviderConfigCiphertext, job.KeyVersion)
		if err == nil {
			var config map[string]string
			err = json.Unmarshal(clear, &config)
			if err == nil {
				err = notification.DefaultProviderRegistry().Validate(job.ChannelKind, config)
				if err == nil {
					values := aggregateTemplateValues(job.Items)
					payload, renderErr := notification.RenderForProvider(job.ChannelKind, job.Template, declaredTemplateValues(values, job.Template.Variables))
					err = renderErr
					if err == nil {
						subject, message := notifyMessage(payload)
						sender := w.Sender
						if sender == nil {
							sender = NotifySender{}
						}
						status, body, retryAfter, err = sender.Send(ctx, job.ChannelKind, config, subject, message)
					}
				}
			}
		}
	}
	finishErr := queue.FinishNotificationDelivery(ctx, job, status, body, retryAfter, err)
	return true, errors.Join(err, finishErr)
}

func declaredTemplateValues(values map[string]string, variables []notification.TemplateVariable) map[string]string {
	declared := make(map[string]string, len(variables))
	for _, variable := range variables {
		if value, exists := values[variable.Name]; exists {
			declared[variable.Name] = value
		}
	}
	return declared
}

func aggregateTemplateValues(items []NotificationDeliveryItem) map[string]string {
	values := make(map[string]string)
	summaries := make([]string, 0, len(items))
	for index, item := range items {
		if index == 0 {
			values["event_type"] = item.EventType
			for _, key := range []string{"severity", "rule_name", "policy_name", "resource_name", "scope_name", "first_observed_at", "last_observed_at", "run_url", "opskeeper_url"} {
				if value, ok := item.Payload[key].(string); ok {
					values[key] = value
				}
			}
			if value, ok := item.Payload["rule"].(string); ok {
				values["rule_name"] = value
			}
		}
		if summary, ok := item.Payload["finding_summary"].(string); ok && summary != "" {
			summaries = append(summaries, notification.SanitizeSummary(summary))
		}
	}
	values["finding_summary"] = strings.Join(summaries, "\n")
	return values
}
