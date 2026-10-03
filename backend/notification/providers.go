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

type ProviderRegistry struct {
	providers map[string]ProviderDescriptor
}

func DefaultProviderRegistry() ProviderRegistry {
	webhook := ProviderDescriptor{
		Kind: "webhook", Name: "HTTPS Webhook", SourceVersion: NotifyProviderMatrixVersion, Supported: true, MaxPayload: 64 << 10, RateLimiting: true,
		Fields: []ProviderField{
			{Name: "url", Label: "Webhook URL", Type: "url", Required: true, Secret: true},
			{Name: "signing_secret", Label: "Signing Secret", Type: "password", Secret: true},
		},
	}
	providers := []ProviderDescriptor{webhook}
	for _, item := range [][2]string{
		{"amazonses", "Amazon SES"}, {"amazonsns", "Amazon SNS"}, {"bark", "Bark"},
		{"dingding", "DingTalk"}, {"discord", "Discord"}, {"mail", "Email"},
		{"fcm", "Firebase Cloud Messaging"}, {"googlechat", "Google Chat"}, {"lark", "Lark"},
		{"line", "Line"}, {"line-notify", "Line Notify"}, {"mailgun", "Mailgun"},
		{"mailtrap", "Mailtrap"}, {"matrix", "Matrix"}, {"mattermost", "Mattermost"},
		{"msteams", "Microsoft Teams"}, {"pagerduty", "PagerDuty"}, {"plivo", "Plivo"},
		{"pushover", "Pushover"}, {"pushbullet", "Pushbullet"}, {"reddit", "Reddit"},
		{"rocketchat", "Rocket.Chat"}, {"sendgrid", "SendGrid"}, {"slack", "Slack"},
		{"syslog", "Syslog"}, {"telegram", "Telegram"}, {"textmagic", "TextMagic"},
		{"twilio", "Twilio"}, {"twitter", "Twitter"}, {"viber", "Viber"},
		{"wechat", "WeChat"}, {"webpush", "Web Push"}, {"whatsapp", "WhatsApp"},
	} {
		providers = append(providers, ProviderDescriptor{Kind: item[0], Name: item[1], SourceVersion: NotifyProviderMatrixVersion, Supported: false})
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
		if field.Name == "url" && value != "" {
			parsed, err := url.ParseRequestURI(value)
			if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Hostname() == "" || parsed.User != nil {
				return fmt.Errorf("webhook URL must be a valid HTTPS URL without embedded credentials")
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
		value, supplied := patch[field.Name]
		if supplied && (!field.Secret || strings.TrimSpace(value) != "") {
			merged[field.Name] = strings.TrimSpace(value)
		}
	}
	return merged
}
