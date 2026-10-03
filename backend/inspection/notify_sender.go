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
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nikoksr/notify"
	notifybark "github.com/nikoksr/notify/service/bark"
	notifydingding "github.com/nikoksr/notify/service/dingding"
	notifydiscord "github.com/nikoksr/notify/service/discord"
	notifyfcm "github.com/nikoksr/notify/service/fcm"
	notifygooglechat "github.com/nikoksr/notify/service/googlechat"
	notifyhttp "github.com/nikoksr/notify/service/http"
	notifylark "github.com/nikoksr/notify/service/lark"
	notifymail "github.com/nikoksr/notify/service/mail"
	notifymailgun "github.com/nikoksr/notify/service/mailgun"
	notifymailtrap "github.com/nikoksr/notify/service/mailtrap"
	notifymatrix "github.com/nikoksr/notify/service/matrix"
	notifymsteams "github.com/nikoksr/notify/service/msteams"
	notifypagerduty "github.com/nikoksr/notify/service/pagerduty"
	notifyplivo "github.com/nikoksr/notify/service/plivo"
	notifypushbullet "github.com/nikoksr/notify/service/pushbullet"
	notifypushover "github.com/nikoksr/notify/service/pushover"
	notifyrocketchat "github.com/nikoksr/notify/service/rocketchat"
	notifysendgrid "github.com/nikoksr/notify/service/sendgrid"
	notifyslack "github.com/nikoksr/notify/service/slack"
	notifytelegram "github.com/nikoksr/notify/service/telegram"
	notifytextmagic "github.com/nikoksr/notify/service/textmagic"
	notifytwilio "github.com/nikoksr/notify/service/twilio"
	notifyviber "github.com/nikoksr/notify/service/viber"
	notifywechat "github.com/nikoksr/notify/service/wechat"

	"github.com/silenceper/wechat/v2/cache"
	"google.golang.org/api/option"
	"maunium.net/go/mautrix/id"
)

type NotifySender struct {
	Client *http.Client
	Now    func() time.Time
}

func notifyMessage(payload []byte) (string, string) {
	var text struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if json.Unmarshal(payload, &text) == nil && (text.Title != "" || text.Body != "") {
		return text.Title, text.Body
	}
	return "OpsKeeper notification", string(payload)
}

func (s NotifySender) Send(ctx context.Context, kind string, config map[string]string, subject, message string) (int, string, time.Duration, error) {
	if kind == "webhook" {
		return s.sendWebhook(ctx, config, subject, message)
	}
	if kind == "fcm" {
		return s.sendFCM(ctx, config, subject, message)
	}
	service, err := s.service(kind, config)
	if err != nil {
		return 0, "", 0, err
	}
	err = notify.NewWithServices(service).Send(ctx, subject, message)
	if err != nil {
		return 0, "", 0, err
	}
	return http.StatusOK, "", 0, nil
}

func (s NotifySender) service(kind string, c map[string]string) (notify.Notifier, error) {
	receivers := splitReceivers(c["receivers"])
	switch kind {
	case "slack":
		n := notifyslack.New(c["api_token"])
		n.AddReceivers(c["channel_id"])
		return n, nil
	case "telegram":
		n, err := notifytelegram.New(c["api_token"])
		if err != nil {
			return nil, err
		}
		id, err := strconv.ParseInt(c["chat_id"], 10, 64)
		if err != nil {
			return nil, err
		}
		n.AddReceivers(id)
		return n, nil
	case "discord":
		n := notifydiscord.New()
		if err := n.AuthenticateWithBotToken(c["bot_token"]); err != nil {
			return nil, err
		}
		n.AddReceivers(c["channel_id"])
		return n, nil
	case "msteams":
		n := notifymsteams.New()
		n.AddReceivers(c["webhook_url"])
		return n, nil
	case "dingding":
		return notifydingding.New(&notifydingding.Config{Token: c["token"], Secret: c["secret"]}), nil
	case "lark":
		return notifylark.NewWebhookService(c["webhook_url"]), nil
	case "mail":
		n := notifymail.New(c["sender_address"], c["smtp_host"]+smtpPort(c["smtp_port"]))
		if c["smtp_user"] != "" {
			n.AuthenticateSMTP("", c["smtp_user"], c["smtp_password"], c["smtp_host"])
		}
		n.BodyFormat(notifymail.PlainText)
		n.AddReceivers(receivers...)
		return n, nil
	case "sendgrid":
		n := notifysendgrid.New(c["api_key"], c["sender_address"], c["sender_name"])
		n.BodyFormat(notifysendgrid.PlainText)
		n.AddReceivers(receivers...)
		return n, nil
	case "mailgun":
		n := notifymailgun.New(c["domain"], c["api_key"], c["sender_address"])
		n.AddReceivers(receivers...)
		return n, nil
	case "mailtrap":
		n := notifymailtrap.New(c["api_key"], c["sender_address"])
		n.AddReceivers(receivers...)
		return n, nil
	case "bark":
		n := notifybark.NewWithServers(c["device_key"], c["server_url"])
		return n, nil
	case "pushover":
		n := notifypushover.New(c["app_token"])
		n.AddReceivers(c["user_key"])
		return n, nil
	case "pushbullet":
		n := notifypushbullet.New(c["api_token"])
		if c["device"] != "" {
			n.AddReceivers(c["device"])
		}
		return n, nil
	case "pagerduty":
		n, err := notifypagerduty.New(c["token"])
		if err != nil {
			return nil, err
		}
		n.SetFromAddress(c["from_address"])
		n.AddReceivers(c["service_id"])
		return n, nil
	case "twilio":
		n, err := notifytwilio.New(c["account_sid"], c["auth_token"], c["from"])
		if err != nil {
			return nil, err
		}
		n.AddReceivers(c["to"])
		return n, nil
	case "plivo":
		n, err := notifyplivo.New(&notifyplivo.ClientOptions{AuthID: c["auth_id"], AuthToken: c["auth_token"]}, &notifyplivo.MessageOptions{Source: c["source"]})
		if err != nil {
			return nil, err
		}
		n.AddReceivers(c["to"])
		return n, nil
	case "textmagic":
		n := notifytextmagic.New(c["username"], c["api_key"])
		n.AddReceivers(c["to"])
		return n, nil
	case "matrix":
		n, err := notifymatrix.New(id.UserID(c["user_id"]), id.RoomID(c["room_id"]), c["home_server"], c["access_token"])
		return n, err
	case "viber":
		n := notifyviber.New(c["app_key"], c["sender_name"], c["sender_avatar"])
		n.AddReceivers(c["user_id"])
		return n, nil
	case "rocketchat":
		u, err := url.Parse(c["server_url"])
		if err != nil {
			return nil, err
		}
		n, err := notifyrocketchat.New(u.Host, u.Scheme, c["user_id"], c["token"])
		if err != nil {
			return nil, err
		}
		n.AddReceivers(c["channel"])
		return n, nil
	case "googlechat":
		n, err := notifygooglechat.New(option.WithCredentialsJSON([]byte(c["credentials_json"])))
		if err != nil {
			return nil, err
		}
		n.AddReceivers(c["space"])
		return n, nil
	case "wechat":
		n := notifywechat.New(&notifywechat.Config{AppID: c["app_id"], AppSecret: c["app_secret"], Token: c["token"], EncodingAESKey: c["encoding_aes_key"], Cache: cache.NewMemory()})
		n.AddReceivers(c["user_id"])
		return n, nil
	default:
		return nil, fmt.Errorf("unsupported notification provider %q", kind)
	}
}

func (s NotifySender) sendFCM(ctx context.Context, c map[string]string, subject, message string) (int, string, time.Duration, error) {
	temp, err := os.CreateTemp("", "opskeeper-fcm-*.json")
	if err != nil {
		return 0, "", 0, err
	}
	path := temp.Name()
	defer os.Remove(path)
	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return 0, "", 0, err
	}
	if _, err := temp.WriteString(c["credentials_json"]); err != nil {
		_ = temp.Close()
		return 0, "", 0, err
	}
	if err := temp.Close(); err != nil {
		return 0, "", 0, err
	}
	n, err := notifyfcm.New(ctx, notifyfcm.WithCredentialsFile(path), notifyfcm.WithProjectID(c["project_id"]))
	if err != nil {
		return 0, "", 0, err
	}
	n.AddReceivers(c["device_token"])
	if err := notify.NewWithServices(n).Send(ctx, subject, message); err != nil {
		return 0, "", 0, err
	}
	return http.StatusOK, "", 0, nil
}

func (s NotifySender) sendWebhook(ctx context.Context, c map[string]string, subject, message string) (int, string, time.Duration, error) {
	target := c["url"]
	parsed, err := url.ParseRequestURI(target)
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Hostname() == "" || parsed.User != nil {
		return 0, "", 0, fmt.Errorf("webhook URL must be a valid HTTPS URL without embedded credentials")
	}
	var status int
	var body string
	var retry time.Duration
	service := notifyhttp.New()
	service.AddReceivers(&notifyhttp.Webhook{URL: target, ContentType: "application/json", Header: http.Header{}, BuildPayload: func(title, msg string) any {
		if json.Valid([]byte(msg)) {
			return json.RawMessage(msg)
		}
		return map[string]string{"title": title, "message": msg}
	}})
	service.PreSend(func(req *http.Request) error {
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			return err
		}
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(raw))
		now := time.Now()
		if s.Now != nil {
			now = s.Now()
		}
		req.Header.Set("X-OpsKeeper-Timestamp", strconv.FormatInt(now.UTC().Unix(), 10))
		if secret := c["signing_secret"]; secret != "" {
			req.Header.Set("X-OpsKeeper-Signature", signWebhook(req.Header.Get("X-OpsKeeper-Timestamp"), raw, []byte(secret)))
		}
		return nil
	})
	service.PostSend(func(_ *http.Request, resp *http.Response) error {
		status = resp.StatusCode
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		body = string(raw)
		retry = parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
		if status < 200 || status >= 300 {
			return fmt.Errorf("webhook returned HTTP %d", status)
		}
		return nil
	})
	if s.Client != nil {
		service.WithClient(s.Client)
	}
	err = notify.NewWithServices(service).Send(ctx, subject, message)
	return status, body, retry, err
}

func splitReceivers(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
func smtpPort(port string) string {
	if port == "" {
		return ""
	}
	return ":" + port
}

func signWebhook(timestamp string, body, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil && at.After(now) {
		return at.Sub(now)
	}
	return 0
}
