//go:build integration

package server

import (
	"context"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"opskeeper/backend/config"
	coremcp "opskeeper/backend/mcp"
)

func TestRealPostgreSQLMCPHealth(t *testing.T) {
	if os.Getenv("OPSK_DB_HOST") == "" {
		t.Skip("OPSK_DB_HOST is required")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	raw := cfg.DatabaseURL
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse PostgreSQL URL: %v", err)
	}
	port, _ := strconv.Atoi(parsed.Port())
	password, _ := parsed.User.Password()
	handler, err := New(Config{Address: "test"})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	endpoint := server.URL + StreamableHTTPPath
	discovery, err := coremcp.Discover(context.Background(), endpoint, 10*time.Second)
	if err != nil || len(discovery.Tools) != len(AvailableTools()) {
		t.Fatalf("Discover() = %#v, %v", discovery, err)
	}
	result, err := coremcp.CallBounded(context.Background(), endpoint, "postgresql_health", map[string]bool{"postgresql_health": true}, map[string]any{"host": parsed.Hostname(), "port": port, "database": parsed.Path[1:], "username": parsed.User.Username(), "password": password, "timeout_seconds": 10}, 10*time.Second, 1<<20)
	if err != nil || len(result) == 0 {
		t.Fatalf("CallBounded(postgresql_health) = %s, %v", result, err)
	}
	result, err = coremcp.CallBounded(context.Background(), endpoint, "postgresql_explain", map[string]bool{"postgresql_explain": true}, map[string]any{"host": parsed.Hostname(), "port": port, "database": parsed.Path[1:], "username": parsed.User.Username(), "password": password, "timeout_seconds": 10, "query": "SELECT 1"}, 10*time.Second, 1<<20)
	if err != nil || len(result) == 0 {
		t.Fatalf("CallBounded(postgresql_explain) = %s, %v", result, err)
	}
}
