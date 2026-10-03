package inspection

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNotifyWebhookRequiresHTTPS(t *testing.T) {
	_, _, _, err := (NotifySender{}).Send(context.Background(), "webhook", map[string]string{"url": "http://example.test/hook"}, "subject", "body")
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("expected HTTPS validation, got %v", err)
	}
}

func TestNotifyWebhookSignsPayload(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("X-OpsKeeper-Signature"), "sha256=") {
			t.Error("signature missing")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	sender := NotifySender{Client: server.Client(), Now: func() time.Time { return time.Unix(100, 0) }}
	status, _, _, err := sender.Send(context.Background(), "webhook", map[string]string{"url": server.URL, "signing_secret": "test"}, "subject", "body")
	if err != nil || status != http.StatusNoContent {
		t.Fatalf("send = %d,%v", status, err)
	}
}

func TestRetryDelayUsesExponentialBackoffAndRetryAfter(t *testing.T) {
	if got := retryDelay(3, 0); got != 8*time.Second {
		t.Fatalf("retryDelay() = %s, want 8s", got)
	}
	if got := retryDelay(1, 2*time.Hour); got != time.Hour {
		t.Fatalf("retryDelay() = %s, want capped 1h", got)
	}
}
