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
	if cfg.LogFormat != "text" {
		t.Fatalf("Load() LogFormat = %q, want text", cfg.LogFormat)
	}
	if cfg.ShutdownTimeout != 10*time.Second || cfg.DependencyTimeout != 2*time.Second {
		t.Fatalf("Load() returned unexpected timeouts: %#v", cfg)
	}
	if cfg.BasePath != "/test-ops" {
		t.Fatalf("Load() returned unexpected base path: %#v", cfg)
	}
	if len(cfg.TrustedProxies) != 0 {
		t.Fatalf("Load() TrustedProxies = %#v, want empty", cfg.TrustedProxies)
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
	for _, format := range []string{"text", "json"} {
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

func TestLoadRejectsInvalidDuration(t *testing.T) {
	t.Setenv("OPSK_SHUTDOWN_TIMEOUT", "later")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid duration error")
	}
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
