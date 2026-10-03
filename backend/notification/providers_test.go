package notification

import (
	"strings"
	"testing"
)

func TestWebhookProviderRejectsInsecureOrUnknownConfig(t *testing.T) {
	registry := DefaultProviderRegistry()
	for _, config := range []map[string]string{
		{"url": "http://example.com/hook"},
		{"url": "https://user:password@example.com/hook"},
		{"url": "https://example.com/hook", "token": "unexpected"},
		{},
	} {
		if err := registry.Validate("webhook", config); err == nil {
			t.Errorf("Validate(%v) succeeded, want rejection", config)
		}
	}
	if err := registry.Validate("missing", map[string]string{"url": "https://example.com/hook"}); err == nil {
		t.Fatal("unknown provider succeeded")
	}
}

func TestProviderPublicConfigRedactsSecretAndMergeKeepsOmittedSecret(t *testing.T) {
	registry := DefaultProviderRegistry()
	current := map[string]string{"url": "https://example.com/private-token"}
	public := registry.PublicConfig("webhook", current)
	if public["url"] != "configured" || strings.Contains(public["url"], "private-token") {
		t.Fatalf("public config exposed secret: %v", public)
	}
	merged := registry.MergeConfig("webhook", current, map[string]string{"url": ""})
	if merged["url"] != current["url"] {
		t.Fatalf("omitted secret was not preserved: %v", merged)
	}
}

func TestRegistryExposesNotifyProvidersAndUnavailableEntries(t *testing.T) {
	registry := DefaultProviderRegistry()
	providers := registry.List()
	if len(providers) < 30 {
		t.Fatalf("registry exposes %d providers, want the published support matrix", len(providers))
	}
	for _, kind := range []string{"webhook", "slack", "telegram", "whatsapp"} {
		provider, ok := registry.Get(kind)
		if !ok {
			t.Fatalf("provider %q missing from registry", kind)
		}
		wantSupported := kind != "whatsapp"
		if provider.Supported != wantSupported {
			t.Errorf("provider %q supported=%t", kind, provider.Supported)
		}
	}
}
