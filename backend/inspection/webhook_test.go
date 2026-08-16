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
