package inspection

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWebhookRequiresHTTPS(t *testing.T) {
	_, _, err := WebhookSender{}.Send(context.Background(), NotificationChannel{Kind: "webhook", WebhookURL: "http://example.test"}, []byte("x"), WebhookEvent{})
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("expected HTTPS validation, got %v", err)
	}
}
func TestWebhookSignsPayload(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("X-OpsKeeper-Signature"), "sha256=") {
			t.Error("signature missing")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	sender := WebhookSender{Client: server.Client(), Now: func() time.Time { return time.Unix(100, 0) }}
	status, _, err := sender.Send(context.Background(), NotificationChannel{Kind: "webhook", WebhookURL: server.URL}, []byte("test"), WebhookEvent{Type: "finding.opened"})
	if err != nil || status != 204 {
		t.Fatalf("send = %d,%v", status, err)
	}
}

func TestWebhookPayloadReturnsRetryAfter(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content type = %q", r.Header.Get("Content-Type"))
		}
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	sender := WebhookSender{Client: server.Client(), Now: func() time.Time { return time.Unix(100, 0) }}
	status, _, delay, err := sender.SendPayload(context.Background(), server.URL, nil, []byte(`{"message":"test"}`))
	if status != http.StatusTooManyRequests || delay != 30*time.Second || err == nil {
		t.Fatalf("SendPayload() = %d, %s, %v, %v", status, delay, delay, err)
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
