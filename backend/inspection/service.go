package inspection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"opskeeper/backend/authorization"
	"opskeeper/backend/notification"
	"opskeeper/backend/persona"
	"opskeeper/backend/resource"
	"opskeeper/backend/secret"
)

type ResourceReader interface {
	Get(context.Context, string) (resource.Resource, error)
}

type PersonaReader interface {
	GetPersona(context.Context, string) (persona.Persona, error)
}

type Service struct {
	store     policyStore
	resources ResourceReader
	personas  PersonaReader
	cipher    secret.Cipher
	providers notification.ProviderRegistry
	tester    NotificationTester
}

type NotificationTester interface {
	Send(context.Context, string, map[string]string, string, string) (int, string, time.Duration, error)
}

type encryptedChannel struct {
	Channel    NotificationChannel
	Ciphertext []byte
	KeyVersion string
}

type notificationChannelStore interface {
	CreateConfiguredChannel(context.Context, NotificationChannel, []byte, string, string) (NotificationChannel, error)
	ListConfiguredChannels(context.Context, string) ([]encryptedChannel, error)
	GetConfiguredChannel(context.Context, string, string) (encryptedChannel, error)
	UpdateConfiguredChannel(context.Context, NotificationChannel, []byte, string, string) (NotificationChannel, error)
	DeleteConfiguredChannel(context.Context, string, string) error
	ReserveChannelTest(context.Context, string) error
}

type policyStore interface {
	CreatePolicy(context.Context, Policy, string) (Policy, error)
	ListPolicies(context.Context, string) ([]Policy, error)
}

func NewService(store policyStore, resources ResourceReader, personas ...PersonaReader) *Service {
	service := &Service{store: store, resources: resources, providers: notification.DefaultProviderRegistry(), tester: NotifySender{}}
	if len(personas) > 0 {
		service.personas = personas[0]
	}
	return service
}

func (s *Service) WithNotificationSecurity(cipher secret.Cipher, providers notification.ProviderRegistry) *Service {
	s.cipher = cipher
	s.providers = providers
	return s
}

func (s *Service) CreatePolicy(ctx context.Context, input Policy, actorID string) (Policy, error) {
	policy, err := normalizePolicy(input)
	if err != nil {
		return Policy{}, err
	}
	if !allowsScope(ctx, policy.ScopeID) {
		return Policy{}, authorization.ErrForbidden
	}
	for _, targetID := range policy.TargetResourceIDs {
		if err := s.validateResource(ctx, policy.ScopeID, targetID, ""); err != nil {
			return Policy{}, err
		}
	}
	if policy.PersonaID != "" {
		if s.personas != nil {
			item, readErr := s.personas.GetPersona(ctx, policy.PersonaID)
			if readErr != nil {
				return Policy{}, readErr
			}
			if item.Status != resource.StatusActive || !allowsScope(ctx, item.ScopeID) {
				return Policy{}, authorization.ErrForbidden
			}
		} else if err := s.validateResource(ctx, policy.ScopeID, policy.PersonaID, "Persona"); err != nil {
			return Policy{}, err
		}
	}
	return s.store.CreatePolicy(ctx, policy, strings.TrimSpace(actorID))
}

func (s *Service) ListPolicies(ctx context.Context, scopeID string) ([]Policy, error) {
	scopeID = strings.TrimSpace(scopeID)
	if scopeID == "" || !allowsScope(ctx, scopeID) {
		return nil, authorization.ErrForbidden
	}
	return s.store.ListPolicies(ctx, scopeID)
}

type operationalStore interface {
	policyStore
	ResolveTargets(context.Context, Policy) ([]string, error)
	CreateManualRun(context.Context, Policy, time.Time, []string) (string, error)
	ListRuns(context.Context, string, int) ([]Run, error)
	ListFindings(context.Context, string, int) ([]Finding, error)
	CreateChannel(context.Context, NotificationChannel) (NotificationChannel, error)
	ListChannels(context.Context, string) ([]NotificationChannel, error)
}

func (s *Service) StartManualRun(ctx context.Context, scopeID, policyID string, now time.Time) (string, error) {
	if !allowsScope(ctx, scopeID) {
		return "", authorization.ErrForbidden
	}
	store, ok := s.store.(operationalStore)
	if !ok {
		return "", invalid("inspection run store is unavailable")
	}
	items, err := store.ListPolicies(ctx, scopeID)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if item.ID == strings.TrimSpace(policyID) {
			if item.Status != PolicyActive {
				return "", ErrConflict
			}
			targets, err := store.ResolveTargets(ctx, item)
			if err != nil {
				return "", err
			}
			if len(targets) == 0 {
				return "", ErrConflict
			}
			item.TargetResourceIDs = targets
			return store.CreateManualRun(ctx, item, now, targets)
		}
	}
	return "", ErrNotFound
}

func (s *Service) ListRuns(ctx context.Context, scopeID string, limit int) ([]Run, error) {
	if !allowsScope(ctx, scopeID) {
		return nil, authorization.ErrForbidden
	}
	store, ok := s.store.(operationalStore)
	if !ok {
		return nil, invalid("inspection run store is unavailable")
	}
	return store.ListRuns(ctx, scopeID, limit)
}
func (s *Service) ListFindings(ctx context.Context, scopeID string, limit int) ([]Finding, error) {
	if !allowsScope(ctx, scopeID) {
		return nil, authorization.ErrForbidden
	}
	store, ok := s.store.(operationalStore)
	if !ok {
		return nil, invalid("inspection run store is unavailable")
	}
	return store.ListFindings(ctx, scopeID, limit)
}
func (s *Service) CreateChannel(ctx context.Context, item NotificationChannel) (NotificationChannel, error) {
	if err := validateNotificationScope(ctx, item.ScopeID); err != nil {
		return NotificationChannel{}, err
	}
	if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.WebhookURL) == "" {
		return NotificationChannel{}, invalid("channel name and webhook URL are required")
	}
	if item.Status == "" {
		item.Status = "active"
	}
	if item.Status != "active" && item.Status != "disabled" {
		return NotificationChannel{}, invalid("invalid channel status")
	}
	if item.RateLimitPerMinute == 0 {
		item.RateLimitPerMinute = 30
	}
	store, ok := s.store.(operationalStore)
	if !ok {
		return NotificationChannel{}, invalid("notification store is unavailable")
	}
	return store.CreateChannel(ctx, item)
}
func (s *Service) ListChannels(ctx context.Context, scopeID string) ([]NotificationChannel, error) {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return nil, err
	}
	if s.cipher != nil {
		store, err := s.channelStore()
		if err != nil {
			return nil, err
		}
		encrypted, err := store.ListConfiguredChannels(ctx, scopeID)
		if err != nil {
			return nil, err
		}
		items := make([]NotificationChannel, 0, len(encrypted))
		for _, item := range encrypted {
			config, err := s.decryptConfig(item)
			if err != nil {
				return nil, fmt.Errorf("decrypt notification channel configuration")
			}
			item.Channel.Config = s.providers.PublicConfig(item.Channel.Kind, config)
			items = append(items, item.Channel)
		}
		return items, nil
	}
	store, ok := s.store.(operationalStore)
	if !ok {
		return nil, invalid("notification store is unavailable")
	}
	return store.ListChannels(ctx, scopeID)
}

func (s *Service) ListNotificationProviders(ctx context.Context, scopeID string) ([]NotificationProvider, error) {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return nil, err
	}
	return s.providers.List(), nil
}

func (s *Service) CreateConfiguredChannel(ctx context.Context, item NotificationChannel, config map[string]string) (NotificationChannel, error) {
	item.ScopeID, item.Name = strings.TrimSpace(item.ScopeID), strings.TrimSpace(item.Name)
	item.Kind = strings.TrimSpace(item.Kind)
	if err := validateNotificationScope(ctx, item.ScopeID); err != nil {
		return NotificationChannel{}, err
	}
	if item.Name == "" || len(item.Name) > 120 {
		return NotificationChannel{}, invalid("channel name must contain 1 to 120 characters")
	}
	if err := s.providers.Validate(item.Kind, config); err != nil {
		return NotificationChannel{}, invalid(err.Error())
	}
	if item.Status == "" {
		item.Status = "active"
	}
	if item.Status != "active" && item.Status != "disabled" {
		return NotificationChannel{}, invalid("invalid channel status")
	}
	if item.RateLimitPerMinute == 0 {
		item.RateLimitPerMinute = 30
	}
	if item.RateLimitPerMinute < 1 || item.RateLimitPerMinute > 600 {
		return NotificationChannel{}, invalid("channel rate limit must be between 1 and 600 per minute")
	}
	store, err := s.channelStore()
	if err != nil {
		return NotificationChannel{}, err
	}
	ciphertext, keyVersion, hash, err := s.encryptConfig(config)
	if err != nil {
		return NotificationChannel{}, err
	}
	created, err := store.CreateConfiguredChannel(ctx, item, ciphertext, keyVersion, hash)
	if err == nil {
		created.Config = s.providers.PublicConfig(created.Kind, config)
	}
	return created, err
}

func (s *Service) UpdateConfiguredChannel(ctx context.Context, scopeID, id string, patch NotificationChannel, config map[string]string) (NotificationChannel, error) {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return NotificationChannel{}, err
	}
	store, err := s.channelStore()
	if err != nil {
		return NotificationChannel{}, err
	}
	current, err := store.GetConfiguredChannel(ctx, id, scopeID)
	if err != nil {
		return NotificationChannel{}, err
	}
	if current.Channel.ScopeID != scopeID {
		return NotificationChannel{}, authorization.ErrForbidden
	}
	currentConfig, err := s.decryptConfig(current)
	if err != nil {
		return NotificationChannel{}, fmt.Errorf("decrypt notification channel configuration")
	}
	merged := s.providers.MergeConfig(current.Channel.Kind, currentConfig, config)
	if err := s.providers.Validate(current.Channel.Kind, merged); err != nil {
		return NotificationChannel{}, invalid(err.Error())
	}
	if patch.Name != "" {
		patch.Name = strings.TrimSpace(patch.Name)
		if patch.Name == "" || len(patch.Name) > 120 {
			return NotificationChannel{}, invalid("channel name must contain 1 to 120 characters")
		}
		current.Channel.Name = patch.Name
	}
	if patch.Status != "" {
		if patch.Status != "active" && patch.Status != "disabled" {
			return NotificationChannel{}, invalid("invalid channel status")
		}
		current.Channel.Status = patch.Status
	}
	if patch.RateLimitPerMinute != 0 {
		if patch.RateLimitPerMinute < 1 || patch.RateLimitPerMinute > 600 {
			return NotificationChannel{}, invalid("channel rate limit must be between 1 and 600 per minute")
		}
		current.Channel.RateLimitPerMinute = patch.RateLimitPerMinute
	}
	if patch.ShareWithChildrenSet {
		current.Channel.ShareWithChildren = patch.ShareWithChildren
	}
	current.Channel.ShareWithChildrenSet = patch.ShareWithChildrenSet
	ciphertext, keyVersion, hash, err := s.encryptConfig(merged)
	if err != nil {
		return NotificationChannel{}, err
	}
	updated, err := store.UpdateConfiguredChannel(ctx, current.Channel, ciphertext, keyVersion, hash)
	if err != nil {
		return NotificationChannel{}, err
	}
	updated.Config = s.providers.PublicConfig(updated.Kind, merged)
	return updated, nil
}

func (s *Service) DeleteConfiguredChannel(ctx context.Context, scopeID, id string) error {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return err
	}
	store, err := s.channelStore()
	if err != nil {
		return err
	}
	return store.DeleteConfiguredChannel(ctx, id, scopeID)
}

func (s *Service) TestConfiguredChannel(ctx context.Context, scopeID, id string) error {
	if err := validateNotificationScope(ctx, scopeID); err != nil {
		return err
	}
	store, err := s.channelStore()
	if err != nil {
		return err
	}
	item, err := store.GetConfiguredChannel(ctx, id, scopeID)
	if err != nil {
		return err
	}
	if item.Channel.Status != "active" {
		return ErrConflict
	}
	config, err := s.decryptConfig(item)
	if err != nil {
		return fmt.Errorf("decrypt notification channel configuration")
	}
	if err := store.ReserveChannelTest(ctx, id); err != nil {
		return err
	}
	if s.tester == nil {
		return invalid("notification test sender is unavailable")
	}
	_, _, _, err = s.tester.Send(ctx, item.Channel.Kind, config, "OpsKeeper notification test", "This is a test notification from OpsKeeper.")
	if err != nil {
		return fmt.Errorf("notification channel test failed")
	}
	return nil
}

func (s *Service) channelStore() (notificationChannelStore, error) {
	store, ok := s.store.(notificationChannelStore)
	if !ok {
		return nil, invalid("notification store is unavailable")
	}
	if s.cipher == nil {
		return nil, invalid("notification configuration encryption is unavailable")
	}
	return store, nil
}

func (s *Service) encryptConfig(config map[string]string) ([]byte, string, string, error) {
	encoded, err := json.Marshal(config)
	if err != nil {
		return nil, "", "", err
	}
	ciphertext, keyVersion, err := s.cipher.Encrypt(encoded)
	if err != nil {
		return nil, "", "", fmt.Errorf("encrypt notification channel configuration")
	}
	hash := sha256.Sum256(encoded)
	return ciphertext, keyVersion, hex.EncodeToString(hash[:]), nil
}

func (s *Service) decryptConfig(item encryptedChannel) (map[string]string, error) {
	if len(item.Ciphertext) == 0 && item.Channel.WebhookURL != "" {
		return map[string]string{"url": item.Channel.WebhookURL}, nil
	}
	encoded, err := s.cipher.Decrypt(item.Ciphertext, item.KeyVersion)
	if err != nil {
		return nil, err
	}
	config := map[string]string{}
	if err := json.Unmarshal(encoded, &config); err != nil {
		return nil, err
	}
	return config, nil
}
func (s *Service) SetPolicyStatus(ctx context.Context, scopeID, policyID, status string) error {
	if !allowsScope(ctx, scopeID) {
		return authorization.ErrForbidden
	}
	if status != PolicyActive && status != PolicyDisabled {
		return invalid("invalid policy status")
	}
	store, ok := s.store.(interface {
		SetPolicyStatus(context.Context, string, string, string) error
	})
	if !ok {
		return invalid("inspection policy store is unavailable")
	}
	return store.SetPolicyStatus(ctx, policyID, scopeID, status)
}

func (s *Service) validateResource(ctx context.Context, scopeID, id, kind string) error {
	if s.resources == nil {
		return invalid("resource service is unavailable")
	}
	item, err := s.resources.Get(ctx, id)
	if err != nil {
		return err
	}
	if item.ScopeID != scopeID || item.Status != resource.StatusActive {
		return authorization.ErrForbidden
	}
	if kind != "" && item.Kind != kind {
		return invalid("policy Persona is not an Persona resource")
	}
	if filter, ok := authorization.ResourceFilterFromContext(ctx); ok && !filter.Allows(item.ScopeID, item.ID) {
		return authorization.ErrForbidden
	}
	return nil
}

func allowsScope(ctx context.Context, scopeID string) bool {
	filter, ok := authorization.ScopeFilterFromContext(ctx)
	return !ok || filter.Allows(scopeID)
}
