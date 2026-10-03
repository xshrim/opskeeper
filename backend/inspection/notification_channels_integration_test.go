//go:build integration

package inspection

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"opskeeper/backend/authorization"
	"opskeeper/backend/migrations"
	"opskeeper/backend/notification"
	"opskeeper/backend/secret"
)

type notificationSenderFunc func(context.Context, string, map[string]string, string, string) (int, string, time.Duration, error)

func (f notificationSenderFunc) Send(ctx context.Context, kind string, config map[string]string, subject, message string) (int, string, time.Duration, error) {
	return f(ctx, kind, config, subject, message)
}

func TestNotificationChannelConfigIsEncryptedAndVersioned(t *testing.T) {
	ctx := context.Background()
	pool := inspectionIntegrationPool(t)
	if err := migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var scopeID string
	if err := pool.QueryRow(ctx, `INSERT INTO scopes(scope_type) VALUES('platform') RETURNING id::text`).Scan(&scopeID); err != nil {
		t.Fatal(err)
	}
	cipher, err := secret.NewLocalEncryptor(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(NewStore(pool), nil).WithNotificationSecurity(cipher, notification.DefaultProviderRegistry())
	service.tester = notificationSenderFunc(func(_ context.Context, kind string, config map[string]string, subject, _ string) (int, string, time.Duration, error) {
		if kind != "webhook" || config["url"] != "https://alerts.example.test/private-path" || subject == "" {
			t.Fatalf("test send got kind=%s config=%v subject=%q", kind, config, subject)
		}
		if config["signing_secret"] != "private-signing-secret" {
			t.Fatalf("test send signing secret=%q", config["signing_secret"])
		}
		return 204, "", 0, nil
	})
	scopeCtx := authorization.WithScopeFilter(ctx, authorization.ScopeFilter{ScopeIDs: []string{scopeID}})
	secretURL := "https://alerts.example.test/private-path"
	channel, err := service.CreateConfiguredChannel(scopeCtx, NotificationChannel{ScopeID: scopeID, Name: "Operations", Kind: "webhook", RateLimitPerMinute: 1}, map[string]string{"url": secretURL, "signing_secret": "private-signing-secret"})
	if err != nil {
		t.Fatal(err)
	}
	if channel.ConfigVersion != 1 || channel.Config["url"] != "configured" || bytes.Contains([]byte(channel.WebhookURL), []byte(secretURL)) {
		t.Fatalf("created channel exposed config or bad version: %+v", channel)
	}
	var storedURL string
	var ciphertext []byte
	if err := pool.QueryRow(ctx, `SELECT channel.webhook_url,version.provider_config_ciphertext FROM notification_channels channel JOIN notification_channel_versions version ON version.channel_id=channel.id WHERE channel.id=$1::uuid`, channel.ID).Scan(&storedURL, &ciphertext); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, []byte(secretURL)) || bytes.Contains(ciphertext, []byte("private-signing-secret")) || storedURL == secretURL {
		t.Fatal("provider secret was stored in plaintext")
	}
	listed, err := service.ListChannels(scopeCtx, scopeID)
	if err != nil || len(listed) != 1 || listed[0].Config["url"] != "configured" {
		t.Fatalf("ListChannels()=%+v, %v", listed, err)
	}
	if err := service.TestConfiguredChannel(scopeCtx, scopeID, channel.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.TestConfiguredChannel(scopeCtx, scopeID, channel.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("second test error=%v, want rate-limit conflict", err)
	}
	updated, err := service.UpdateConfiguredChannel(scopeCtx, scopeID, channel.ID, NotificationChannel{Status: "disabled"}, nil)
	if err != nil || updated.ConfigVersion != 2 || updated.Status != "disabled" {
		t.Fatalf("UpdateConfiguredChannel()=%+v, %v", updated, err)
	}
	if err := service.DeleteConfiguredChannel(scopeCtx, scopeID, channel.ID); err != nil {
		t.Fatal(err)
	}
	listed, err = service.ListChannels(scopeCtx, scopeID)
	if err != nil || len(listed) != 0 {
		t.Fatalf("deleted channel remains visible: %+v, %v", listed, err)
	}
}
