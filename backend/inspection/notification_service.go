package inspection

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"opskeeper/backend/notification"
)

func (s *Service) CreateNotificationTemplate(ctx context.Context, scopeID, name, actorID string, shareWithChildren bool, draft notification.TemplateDraft) (NotificationTemplateRecord, error) {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return NotificationTemplateRecord{}, err
	}
	store, err := s.notificationStore()
	if err != nil {
		return NotificationTemplateRecord{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 120 {
		return NotificationTemplateRecord{}, invalid("template name must contain 1 to 120 characters")
	}
	item, err := store.CreateNotificationTemplate(ctx, scopeID, name, actorID, shareWithChildren, draft)
	if err != nil {
		if errors.Is(err, ErrConflict) || errors.Is(err, ErrNotFound) {
			return NotificationTemplateRecord{}, err
		}
		if strings.Contains(err.Error(), "invalid notification template") {
			return NotificationTemplateRecord{}, invalid(err.Error())
		}
		return NotificationTemplateRecord{}, err
	}
	return item, nil
}

func (s *Service) ListNotificationTemplates(ctx context.Context, scopeID string) ([]NotificationTemplateRecord, error) {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return nil, err
	}
	store, err := s.notificationStore()
	if err != nil {
		return nil, err
	}
	return store.ListNotificationTemplates(ctx, scopeID)
}

func (s *Service) CreateNotificationTemplateVersion(ctx context.Context, scopeID, templateID, actorID string, draft notification.TemplateDraft) (notification.TemplateVersion, error) {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return notification.TemplateVersion{}, err
	}
	store, err := s.notificationStore()
	if err != nil {
		return notification.TemplateVersion{}, err
	}
	item, err := store.CreateNotificationTemplateVersion(ctx, templateID, scopeID, actorID, draft)
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict) {
		return notification.TemplateVersion{}, err
	}
	if err != nil && strings.Contains(err.Error(), "invalid notification template") {
		return notification.TemplateVersion{}, invalid(err.Error())
	}
	return item, err
}

func (s *Service) PreviewNotificationTemplate(ctx context.Context, scopeID, templateID, versionID string, values map[string]string) (notification.RenderedTemplate, error) {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return notification.RenderedTemplate{}, err
	}
	store, err := s.notificationStore()
	if err != nil {
		return notification.RenderedTemplate{}, err
	}
	version, err := store.GetNotificationTemplateVersion(ctx, templateID, scopeID, versionID)
	if err != nil {
		return notification.RenderedTemplate{}, err
	}
	if version.Status != "draft" && version.Status != "published" {
		return notification.RenderedTemplate{}, ErrNotFound
	}
	item, err := notification.RenderTemplate(version.Draft, values)
	if err != nil {
		return notification.RenderedTemplate{}, invalid(err.Error())
	}
	return item, nil
}

func (s *Service) PublishNotificationTemplate(ctx context.Context, scopeID, templateID, versionID string) (notification.TemplateVersion, error) {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return notification.TemplateVersion{}, err
	}
	store, err := s.notificationStore()
	if err != nil {
		return notification.TemplateVersion{}, err
	}
	return store.PublishNotificationTemplate(ctx, templateID, scopeID, versionID)
}

func (s *Service) DeleteNotificationTemplate(ctx context.Context, scopeID, id string) error {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return err
	}
	store, err := s.notificationStore()
	if err != nil {
		return err
	}
	return store.DeleteNotificationTemplate(ctx, scopeID, id)
}

func (s *Service) SetNotificationTemplateSharing(ctx context.Context, scopeID, id string, share bool) error {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return err
	}
	store, err := s.notificationStore()
	if err != nil {
		return err
	}
	return store.SetNotificationTemplateSharing(ctx, scopeID, id, share)
}

func (s *Service) CreateNotificationRule(ctx context.Context, scopeID, actorID string, item NotificationRuleRecord) (NotificationRuleRecord, error) {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return NotificationRuleRecord{}, err
	}
	store, err := s.notificationStore()
	if err != nil {
		return NotificationRuleRecord{}, err
	}
	return store.CreateNotificationRule(ctx, scopeID, actorID, item)
}

func (s *Service) ListNotificationRules(ctx context.Context, scopeID string) ([]NotificationRuleRecord, error) {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return nil, err
	}
	store, err := s.notificationStore()
	if err != nil {
		return nil, err
	}
	return store.ListNotificationRules(ctx, scopeID)
}

func (s *Service) TestNotificationRule(ctx context.Context, scopeID, id string) (int, error) {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return 0, err
	}
	store, err := s.notificationStore()
	if err != nil {
		return 0, err
	}
	routes, err := store.GetNotificationRuleRoutes(ctx, scopeID, id)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, route := range routes {
		if err := s.testNotificationRoute(ctx, scopeID, route); err != nil {
			return count, err
		}
		count++
	}
	if count == 0 {
		return 0, invalid("notification rule has no active delivery routes")
	}
	return count, nil
}

func (s *Service) testNotificationRoute(ctx context.Context, scopeID string, route NotificationRoute) error {
	channelStore, err := s.channelStore()
	if err != nil {
		return err
	}
	channel, err := channelStore.GetConfiguredChannel(ctx, route.ChannelID, scopeID)
	if err != nil {
		return err
	}
	if channel.Channel.Status != "active" {
		return ErrConflict
	}
	store, err := s.notificationStore()
	if err != nil {
		return err
	}
	version, err := store.GetNotificationTemplateVersionByID(ctx, scopeID, route.TemplateVersionID)
	if err != nil {
		return err
	}
	payload, err := notification.RenderForProvider(channel.Channel.Kind, version.Draft, notificationTestValues(version.Draft))
	if err != nil {
		return invalid(err.Error())
	}
	config, err := s.decryptConfig(channel)
	if err != nil {
		return fmt.Errorf("decrypt notification channel configuration")
	}
	if err := channelStore.ReserveChannelTest(ctx, route.ChannelID); err != nil {
		return err
	}
	if s.tester == nil {
		return invalid("notification test sender is unavailable")
	}
	channel.Channel.WebhookURL = config["url"]
	_, _, err = s.tester.Send(ctx, channel.Channel, []byte(config["signing_secret"]), WebhookEvent{Type: "notification.test", Data: payload})
	if err != nil {
		return fmt.Errorf("notification rule test failed")
	}
	return nil
}

func notificationTestValues(draft notification.TemplateDraft) map[string]string {
	values := map[string]string{
		"event_type": "notification.test", "severity": "warning", "rule_name": "Test notification rule",
		"policy_name": "Test inspection policy", "resource_name": "test-resource", "scope_name": "test-scope",
		"finding_summary": "Test notification route", "first_observed_at": "2026-01-01T00:00:00Z",
		"last_observed_at": "2026-01-01T00:00:00Z", "run_url": "https://example.invalid/runs/test",
		"opskeeper_url": "https://example.invalid",
	}
	declared := make(map[string]string, len(draft.Variables))
	for _, variable := range draft.Variables {
		declared[variable.Name] = values[variable.Name]
	}
	return declared
}

func (s *Service) UpdateNotificationRule(ctx context.Context, scopeID, id string, item NotificationRuleRecord) (NotificationRuleRecord, error) {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return NotificationRuleRecord{}, err
	}
	store, err := s.notificationStore()
	if err != nil {
		return NotificationRuleRecord{}, err
	}
	return store.UpdateNotificationRule(ctx, scopeID, id, item)
}

func (s *Service) DeleteNotificationRule(ctx context.Context, scopeID, id string) error {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return err
	}
	store, err := s.notificationStore()
	if err != nil {
		return err
	}
	return store.DisableNotificationRule(ctx, scopeID, id)
}

func (s *Service) SetPolicyNotificationRules(ctx context.Context, scopeID, policyID string, ruleIDs []string) ([]string, error) {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return nil, err
	}
	store, err := s.notificationStore()
	if err != nil {
		return nil, err
	}
	return store.SetPolicyNotificationRules(ctx, scopeID, policyID, ruleIDs)
}

func (s *Service) ListPolicyNotificationRules(ctx context.Context, scopeID, policyID string) ([]string, error) {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return nil, err
	}
	store, err := s.notificationStore()
	if err != nil {
		return nil, err
	}
	return store.ListPolicyNotificationRules(ctx, scopeID, policyID)
}

func (s *Service) ListNotificationDeliveries(ctx context.Context, scopeID, status, eventType string, limit int) ([]NotificationDeliveryRecord, error) {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return nil, err
	}
	store, err := s.notificationStore()
	if err != nil {
		return nil, err
	}
	return store.ListNotificationDeliveries(ctx, scopeID, status, eventType, limit)
}

func (s *Service) RetryNotificationDelivery(ctx context.Context, scopeID, id string) error {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return err
	}
	store, err := s.notificationStore()
	if err != nil {
		return err
	}
	return store.RetryNotificationDelivery(ctx, scopeID, id)
}

func (s *Service) notificationStore() (notificationCatalogStore, error) {
	store, ok := s.store.(notificationCatalogStore)
	if !ok {
		return nil, invalid("notification store is unavailable")
	}
	return store, nil
}
