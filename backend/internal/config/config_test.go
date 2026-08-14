package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
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
	if cfg.ShutdownTimeout != 10*time.Second || cfg.DependencyTimeout != 2*time.Second {
		t.Fatalf("Load() returned unexpected timeouts: %#v", cfg)
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	t.Setenv("OPSK_SHUTDOWN_TIMEOUT", "later")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid duration error")
	}
}
