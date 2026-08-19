package config

import (
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("OPSK_BASE_PATH", "/test-ops")
	t.Setenv("OPSK_ENVIRONMENT", "test")
	t.Setenv("OPSK_HTTP_ADDRESS", ":9090")
	t.Setenv("OPSK_DATABASE_URL", "postgres://test")
	t.Setenv("OPSK_REDIS_URL", "redis://test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Environment != "test" || cfg.HTTPAddress != ":9090" {
		t.Fatalf("Load() returned unexpected config: %#v", cfg)
	}
	if cfg.LogFormat != "raw" {
		t.Fatalf("Load() LogFormat = %q, want raw", cfg.LogFormat)
	}
	if !cfg.LogHealthIgnore {
		t.Fatalf("Load() LogHealthIgnore = false, want true")
	}
	if cfg.ShutdownTimeout != 10*time.Second || cfg.DependencyTimeout != 2*time.Second {
		t.Fatalf("Load() returned unexpected timeouts: %#v", cfg)
	}
	if cfg.BasePath != "/test-ops" {
		t.Fatalf("Load() returned unexpected base path: %#v", cfg)
	}
	if cfg.CookieSecure || cfg.SessionAccessTTL != 15*time.Minute || cfg.SessionRefreshTTL != 7*24*time.Hour {
		t.Fatalf("Load() returned unexpected session config: %#v", cfg)
	}
	if len(cfg.TrustedProxies) != 0 {
		t.Fatalf("Load() TrustedProxies = %#v, want empty", cfg.TrustedProxies)
	}
	if cfg.ConnectorTimeout != 10*time.Second || cfg.ConnectorMaxConcurrency != 8 || cfg.ConnectorMaxResponseBytes != 4<<20 {
		t.Fatalf("Load() returned unexpected connector config: %#v", cfg)
	}
	if cfg.HTTPMaxBodyBytes != 2<<20 || cfg.HTTPRateLimitPerMinute != 600 || len(cfg.AllowedOrigins) != 0 {
		t.Fatalf("Load() returned unexpected HTTP security config: %#v", cfg)
	}
}

func TestLoadAcceptsHTTPSOriginsAndHTTPLimits(t *testing.T) {
	t.Setenv("OPSK_ALLOWED_ORIGINS", "https://ops.example.com, http://localhost:5173, https://OPS.EXAMPLE.COM")
	t.Setenv("OPSK_HTTP_MAX_BODY_BYTES", "4096")
	t.Setenv("OPSK_HTTP_RATE_LIMIT_PER_MINUTE", "120")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "https://otel.example.com")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.HTTPMaxBodyBytes != 4096 || cfg.HTTPRateLimitPerMinute != 120 {
		t.Fatalf("Load() HTTP config = %#v", cfg)
	}
	if cfg.OTLPExporterEndpoint != "https://otel.example.com" {
		t.Fatalf("Load() telemetry endpoint = %q", cfg.OTLPExporterEndpoint)
	}
}

func TestLoadRejectsInvalidOriginsAndHTTPLimits(t *testing.T) {
	for _, test := range []struct{ key, value string }{
		{key: "OPSK_ALLOWED_ORIGINS", value: "https://example.com/path"},
		{key: "OPSK_ALLOWED_ORIGINS", value: "javascript:alert(1)"},
		{key: "OPSK_HTTP_MAX_BODY_BYTES", value: "100"},
		{key: "OPSK_HTTP_RATE_LIMIT_PER_MINUTE", value: "0"},
	} {
		t.Run(test.key+test.value, func(t *testing.T) {
			t.Setenv(test.key, test.value)
			if _, err := Load(); err == nil {
				t.Fatalf("Load() error = nil for %s=%q", test.key, test.value)
			}
		})
	}
}

func TestLoadAcceptsConnectorLimits(t *testing.T) {
	t.Setenv("OPSK_CONNECTOR_TIMEOUT", "3s")
	t.Setenv("OPSK_CONNECTOR_MAX_CONCURRENCY", "12")
	t.Setenv("OPSK_CONNECTOR_MAX_RESPONSE_BYTES", "2097152")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ConnectorTimeout != 3*time.Second || cfg.ConnectorMaxConcurrency != 12 || cfg.ConnectorMaxResponseBytes != 2097152 {
		t.Fatalf("Load() connector config = %#v", cfg)
	}
}

func TestLoadRejectsInvalidConnectorLimits(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{key: "OPSK_CONNECTOR_TIMEOUT", value: "0s"},
		{key: "OPSK_CONNECTOR_MAX_CONCURRENCY", value: "0"},
		{key: "OPSK_CONNECTOR_MAX_RESPONSE_BYTES", value: "128"},
	}
	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			t.Setenv(test.key, test.value)
			if _, err := Load(); err == nil {
				t.Fatalf("Load() error = nil for %s=%q", test.key, test.value)
			}
		})
	}
}

func TestLoadAcceptsTrustedProxies(t *testing.T) {
	t.Setenv("OPSK_TRUSTED_PROXIES", "10.0.0.0/8, 192.0.2.10, 2001:db8::/32, 192.0.2.10/32")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("192.0.2.10/32"),
		netip.MustParsePrefix("2001:db8::/32"),
	}
	if len(cfg.TrustedProxies) != len(want) {
		t.Fatalf("Load() TrustedProxies = %#v, want %#v", cfg.TrustedProxies, want)
	}
	for index := range want {
		if cfg.TrustedProxies[index] != want[index] {
			t.Fatalf("Load() TrustedProxies[%d] = %v, want %v", index, cfg.TrustedProxies[index], want[index])
		}
	}
}

func TestLoadRejectsInvalidTrustedProxy(t *testing.T) {
	for _, value := range []string{"10.0.0.0/99", "proxy.example.com", "10.0.0.1,,10.0.0.2"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("OPSK_TRUSTED_PROXIES", value)
			if _, err := Load(); err == nil {
				t.Fatalf("Load() error = nil for OPSK_TRUSTED_PROXIES=%q", value)
			}
		})
	}
}

func TestLoadAcceptsLogFormats(t *testing.T) {
	for _, format := range []string{"raw", "text", "json"} {
		t.Run(format, func(t *testing.T) {
			t.Setenv("OPSK_LOG_FORMAT", format)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.LogFormat != format {
				t.Fatalf("Load() LogFormat = %q, want %q", cfg.LogFormat, format)
			}
		})
	}
}

func TestLoadRejectsInvalidLogFormat(t *testing.T) {
	t.Setenv("OPSK_LOG_FORMAT", "pretty")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid log format error")
	}
}

func TestLoadHealthRequestLogging(t *testing.T) {
	t.Setenv("OPSK_LOG_HEALTH_IGNORE", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LogHealthIgnore {
		t.Fatal("Load() LogHealthIgnore = true, want false")
	}

	t.Setenv("OPSK_LOG_HEALTH_IGNORE", "sometimes")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil for invalid OPSK_LOG_HEALTH_IGNORE")
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	t.Setenv("OPSK_SHUTDOWN_TIMEOUT", "later")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid duration error")
	}
}

func TestLoadRequiresSecureCookiesInProduction(t *testing.T) {
	t.Setenv("OPSK_ENVIRONMENT", "production")
	t.Setenv("OPSK_DATABASE_URL", "postgres://production-db/opskeeper")
	t.Setenv("OPSK_REDIS_URL", "rediss://production-redis/0")
	t.Setenv("OPSK_COOKIE_SECURE", "false")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil for insecure production cookies")
	}

	t.Setenv("OPSK_COOKIE_SECURE", "true")
	cfg, err := Load()
	if err != nil || !cfg.CookieSecure {
		t.Fatalf("Load() = %#v, %v", cfg, err)
	}
}

func TestLoadRejectsDevelopmentDefaultsInProduction(t *testing.T) {
	t.Setenv("OPSK_ENVIRONMENT", "production")
	t.Setenv("OPSK_COOKIE_SECURE", "true")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil for production development database defaults")
	}

	t.Setenv("OPSK_DATABASE_URL", "postgres://production-db/opskeeper")
	t.Setenv("OPSK_REDIS_URL", "rediss://production-redis/0")
	t.Setenv("OPSK_ALLOWED_ORIGINS", "http://ops.example.com")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil for production HTTP origin")
	}
}

func TestLoadRejectsInvalidSessionConfiguration(t *testing.T) {
	for _, test := range []struct {
		name  string
		key   string
		value string
	}{
		{name: "cookie boolean", key: "OPSK_COOKIE_SECURE", value: "sometimes"},
		{name: "access duration", key: "OPSK_SESSION_ACCESS_TTL", value: "soon"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(test.key, test.value)
			if _, err := Load(); err == nil {
				t.Fatalf("Load() error = nil for %s=%q", test.key, test.value)
			}
		})
	}

	t.Run("refresh must exceed access", func(t *testing.T) {
		t.Setenv("OPSK_SESSION_ACCESS_TTL", "1h")
		t.Setenv("OPSK_SESSION_REFRESH_TTL", "1h")
		if _, err := Load(); err == nil {
			t.Fatal("Load() error = nil for equal session TTLs")
		}
	})
}

func TestLoadAcceptsBasePaths(t *testing.T) {
	for _, basePath := range []string{"/", "/opskeeper", "/platform/opskeeper"} {
		t.Run(basePath, func(t *testing.T) {
			t.Setenv("OPSK_BASE_PATH", basePath)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.BasePath != basePath {
				t.Fatalf("Load() BasePath = %q, want %q", cfg.BasePath, basePath)
			}
		})
	}
}

func TestLoadRejectsInvalidBasePath(t *testing.T) {
	invalidBasePaths := []string{"", "opskeeper", "OpsKeeper", "/ops_keeper", "/opskeeper/", "//opskeeper", "/-opskeeper", "/opskeeper-"}
	invalidBasePaths = append(invalidBasePaths, "/"+strings.Repeat("a", maxBasePathLength))
	for _, basePath := range invalidBasePaths {
		t.Run(basePath, func(t *testing.T) {
			t.Setenv("OPSK_BASE_PATH", basePath)
			if _, err := Load(); err == nil {
				t.Fatalf("Load() error = nil for OPSK_BASE_PATH=%q", basePath)
			}
		})
	}
}
