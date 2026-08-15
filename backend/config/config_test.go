package config

import (
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
