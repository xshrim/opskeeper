package inspection

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WebhookSender delivers one immutable inspection event. Persistence and retry
// scheduling belong to the queue store, keeping HTTP I/O outside transactions.
type WebhookSender struct {
	Client *http.Client
	Now    func() time.Time
}
type WebhookEvent struct {
	Type       string    `json:"type"`
	Finding    Finding   `json:"finding"`
	RunID      string    `json:"run_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

func (s WebhookSender) Send(ctx context.Context, channel NotificationChannel, secret []byte, event WebhookEvent) (int, string, error) {
	if channel.Kind != "webhook" {
		return 0, "", invalid("notification channel is not a webhook")
	}
	if !strings.HasPrefix(strings.ToLower(channel.WebhookURL), "https://") {
		return 0, "", invalid("webhook URL must use HTTPS")
	}
	if s.Client == nil {
		s.Client = &http.Client{Timeout: 10 * time.Second}
	}
	if s.Now == nil {
		s.Now = time.Now
	}
	event.OccurredAt = s.Now().UTC()
	body, err := json.Marshal(event)
	if err != nil {
		return 0, "", err
	}
	timestamp := fmt.Sprintf("%d", event.OccurredAt.Unix())
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, channel.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OpsKeeper-Timestamp", timestamp)
	req.Header.Set("X-OpsKeeper-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	resp, err := s.Client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, string(raw), fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}
	return resp.StatusCode, string(raw), nil
}
