package notification

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

const NotifyProviderMatrixVersion = "v1.6.0"

type ProviderField struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Secret   bool   `json:"secret"`
}
type ProviderDescriptor struct {
	Kind          string          `json:"kind"`
	Name          string          `json:"name"`
	SourceVersion string          `json:"source_version"`
	Supported     bool            `json:"supported"`
	Fields        []ProviderField `json:"fields"`
	MaxPayload    int             `json:"max_payload_bytes"`
	RateLimiting  bool            `json:"rate_limiting"`
}
type ProviderRegistry struct{ providers map[string]ProviderDescriptor }

func field(name, label, typ string, required, secret bool) ProviderField {
	return ProviderField{Name: name, Label: label, Type: typ, Required: required, Secret: secret}
}

func DefaultProviderRegistry() ProviderRegistry {
	available := func(kind, name string, fields []ProviderField) ProviderDescriptor {
		return ProviderDescriptor{Kind: kind, Name: name, SourceVersion: NotifyProviderMatrixVersion, Supported: true, Fields: fields, MaxPayload: 64 << 10, RateLimiting: true}
	}
	unavailable := func(kind, name string) ProviderDescriptor {
		return ProviderDescriptor{Kind: kind, Name: name, SourceVersion: NotifyProviderMatrixVersion, MaxPayload: 64 << 10, RateLimiting: true}
	}
	providers := []ProviderDescriptor{
		available("webhook", "HTTPS Webhook", []ProviderField{field("url", "Webhook URL", "url", true, true), field("signing_secret", "Signing Secret", "password", false, true)}),
		available("slack", "Slack", []ProviderField{field("api_token", "Bot Token", "password", true, true), field("channel_id", "Channel ID", "string", true, false)}),
		available("telegram", "Telegram", []ProviderField{field("api_token", "Bot Token", "password", true, true), field("chat_id", "Chat ID", "string", true, false)}),
		available("discord", "Discord", []ProviderField{field("bot_token", "Bot Token", "password", true, true), field("channel_id", "Channel ID", "string", true, false)}),
		available("msteams", "Microsoft Teams", []ProviderField{field("webhook_url", "Incoming Webhook URL", "url", true, true)}),
		available("dingding", "DingTalk", []ProviderField{field("token", "Access Token", "password", true, true), field("secret", "Signing Secret", "password", true, true)}),
		available("lark", "Lark", []ProviderField{field("webhook_url", "Webhook URL", "url", true, true)}),
		available("mail", "SMTP Email", []ProviderField{field("sender_address", "Sender Address", "email", true, false), field("smtp_host", "SMTP Host", "string", true, false), field("smtp_port", "SMTP Port", "number", false, false), field("receivers", "Recipients", "string", true, false), field("smtp_user", "SMTP User", "string", false, false), field("smtp_password", "SMTP Password", "password", false, true)}),
		available("sendgrid", "SendGrid", []ProviderField{field("api_key", "API Key", "password", true, true), field("sender_address", "Sender Address", "email", true, false), field("sender_name", "Sender Name", "string", false, false), field("receivers", "Recipients", "string", true, false)}),
		available("mailgun", "Mailgun", []ProviderField{field("domain", "Domain", "string", true, false), field("api_key", "API Key", "password", true, true), field("sender_address", "Sender Address", "email", true, false), field("receivers", "Recipients", "string", true, false)}),
		available("mailtrap", "Mailtrap", []ProviderField{field("api_key", "API Key", "password", true, true), field("sender_address", "Sender Address", "email", true, false), field("receivers", "Recipients", "string", true, false)}),
		available("bark", "Bark", []ProviderField{field("device_key", "Device Key", "password", true, true), field("server_url", "Server URL", "url", false, false)}),
		available("pushover", "Pushover", []ProviderField{field("app_token", "App Token", "password", true, true), field("user_key", "User or Group Key", "string", true, false)}),
		available("pushbullet", "Pushbullet", []ProviderField{field("api_token", "API Token", "password", true, true), field("device", "Device Nickname", "string", false, false)}),
		available("pagerduty", "PagerDuty", []ProviderField{field("token", "Access Token", "password", true, true), field("from_address", "From Address", "email", true, false), field("service_id", "Service ID", "string", true, false)}),
		available("twilio", "Twilio SMS", []ProviderField{field("account_sid", "Account SID", "password", true, true), field("auth_token", "Auth Token", "password", true, true), field("from", "From Number", "string", true, false), field("to", "Recipient Number", "string", true, false)}),
		available("plivo", "Plivo SMS", []ProviderField{field("auth_id", "Auth ID", "password", true, true), field("auth_token", "Auth Token", "password", true, true), field("source", "Source Number", "string", true, false), field("to", "Recipient Number", "string", true, false)}),
		available("textmagic", "TextMagic", []ProviderField{field("username", "Username", "string", true, false), field("api_key", "API Key", "password", true, true), field("to", "Recipient Number", "string", true, false)}),
		available("matrix", "Matrix", []ProviderField{field("user_id", "User ID", "string", true, false), field("room_id", "Room ID", "string", true, false), field("home_server", "Home Server", "url", true, false), field("access_token", "Access Token", "password", true, true)}),
		available("wechat", "WeChat Official Account", []ProviderField{field("app_id", "App ID", "password", true, true), field("app_secret", "App Secret", "password", true, true), field("token", "Token", "password", true, true), field("encoding_aes_key", "Encoding AES Key", "password", false, true), field("user_id", "User ID", "string", true, false)}),
		available("viber", "Viber", []ProviderField{field("app_key", "App Key", "password", true, true), field("sender_name", "Sender Name", "string", true, false), field("sender_avatar", "Sender Avatar", "url", false, false), field("user_id", "User ID", "string", true, false)}),
		available("rocketchat", "Rocket.Chat", []ProviderField{field("server_url", "Server URL", "url", true, false), field("user_id", "User ID", "string", true, false), field("token", "Personal Access Token", "password", true, true), field("channel", "Channel", "string", true, false)}),
		available("googlechat", "Google Chat", []ProviderField{field("credentials_json", "Service Account JSON", "textarea", true, true), field("space", "Space", "string", true, false)}),
		available("fcm", "Firebase Cloud Messaging", []ProviderField{field("credentials_json", "Service Account JSON", "textarea", true, true), field("project_id", "Project ID", "string", true, false), field("device_token", "Device Token", "string", true, false)}),
		unavailable("amazonses", "Amazon SES"), unavailable("amazonsns", "Amazon SNS"), unavailable("line", "LINE Messaging"), unavailable("line-notify", "LINE Notify (service retired)"), unavailable("mattermost", "Mattermost"), unavailable("reddit", "Reddit"), unavailable("syslog", "Syslog"), unavailable("twitter", "Twitter"), unavailable("webpush", "Web Push"), unavailable("whatsapp", "WhatsApp (notify placeholder)"),
	}
	registry := ProviderRegistry{providers: make(map[string]ProviderDescriptor, len(providers))}
	for _, provider := range providers {
		registry.providers[provider.Kind] = provider
	}
	return registry
}

func (r ProviderRegistry) List() []ProviderDescriptor {
	items := make([]ProviderDescriptor, 0, len(r.providers))
	for _, provider := range r.providers {
		items = append(items, provider)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Kind < items[j].Kind })
	return items
}
func (r ProviderRegistry) Get(kind string) (ProviderDescriptor, bool) {
	provider, ok := r.providers[strings.TrimSpace(kind)]
	return provider, ok
}

func (r ProviderRegistry) Validate(kind string, config map[string]string) error {
	provider, ok := r.Get(kind)
	if !ok || !provider.Supported {
		return fmt.Errorf("unsupported notification provider")
	}
	for _, field := range provider.Fields {
		value := strings.TrimSpace(config[field.Name])
		if field.Required && value == "" {
			return fmt.Errorf("provider field %q is required", field.Name)
		}
		if field.Type == "url" && value != "" {
			parsed, err := url.ParseRequestURI(value)
			if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" || parsed.User != nil {
				return fmt.Errorf("provider field %q must be a valid URL without embedded credentials", field.Name)
			}
			if kind == "webhook" && !strings.EqualFold(parsed.Scheme, "https") {
				return fmt.Errorf("webhook URL must use HTTPS")
			}
		}
	}
	for name := range config {
		known := false
		for _, field := range provider.Fields {
			if field.Name == name {
				known = true
				break
			}
		}
		if !known {
			return fmt.Errorf("unknown provider field %q", name)
		}
	}
	return nil
}

func (r ProviderRegistry) PublicConfig(kind string, config map[string]string) map[string]string {
	provider, ok := r.Get(kind)
	if !ok {
		return map[string]string{}
	}
	public := make(map[string]string, len(config))
	for _, field := range provider.Fields {
		if field.Secret && strings.TrimSpace(config[field.Name]) != "" {
			public[field.Name] = "configured"
		} else if !field.Secret {
			public[field.Name] = config[field.Name]
		}
	}
	return public
}
func (r ProviderRegistry) MergeConfig(kind string, current, patch map[string]string) map[string]string {
	merged := make(map[string]string, len(current)+len(patch))
	for key, value := range current {
		merged[key] = value
	}
	provider, ok := r.Get(kind)
	if !ok {
		return merged
	}
	for _, field := range provider.Fields {
		if value, supplied := patch[field.Name]; supplied && (!field.Secret || strings.TrimSpace(value) != "") {
			merged[field.Name] = strings.TrimSpace(value)
		}
	}
	return merged
}
