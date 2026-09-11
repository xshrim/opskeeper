package host

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestResolveInputUsesToolThenEnvironmentThenLocal(t *testing.T) {
	t.Setenv("HOST_MCP_HOST", "env.example")
	t.Setenv("HOST_MCP_USERNAME", "env-user")
	t.Setenv("HOST_MCP_PASSWORD", "env-password")
	input, target, err := resolveInput(ConnectionInput{Host: "tool.example", Username: "tool-user", Password: "tool-password"})
	if err != nil {
		t.Fatal(err)
	}
	if target.Mode != "ssh" || input.Host != "tool.example" || input.Username != "tool-user" {
		t.Fatalf("tool input did not win: input=%+v target=%+v", input, target)
	}
	_, target, err = resolveInput(ConnectionInput{})
	if err != nil {
		t.Fatal(err)
	}
	if target.Mode != "ssh" || target.Host != "env.example" {
		t.Fatalf("environment input not selected: %+v", target)
	}
	t.Setenv("HOST_MCP_HOST", "")
	if _, target, err = resolveInput(ConnectionInput{}); err != nil || target.Mode != "local" {
		t.Fatalf("local fallback = %+v, %v", target, err)
	}
}

func TestFileLogsSupportsTailSinceAndKeyword(t *testing.T) {
	file, err := os.CreateTemp("", "opskeeper-host-log-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())
	_, _ = file.WriteString("2026-01-01T00:00:00Z info ready\n2026-01-01T00:01:00Z error timeout\n2026-01-01T00:02:00Z info done\n")
	_ = file.Close()
	output, err := FileLogs(context.Background(), FileLogsInput{Path: file.Name(), Tail: "1", Keyword: "error"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.Logs, "timeout") || strings.Contains(output.Logs, "ready") {
		t.Fatalf("unexpected log output: %q", output.Logs)
	}
}

func TestKnownHostsRejectsPath(t *testing.T) {
	if _, _, err := knownHostsCallback("/tmp/known_hosts"); err == nil {
		t.Fatal("expected known_hosts path to be rejected")
	}
}

func TestPrivateKeyRejectsPath(t *testing.T) {
	if _, err := privateKeyBytes("id_rsa"); err == nil {
		t.Fatal("expected private key path to be rejected")
	}
}

func TestParseCPUTicksIgnoresBlankLines(t *testing.T) {
	ticks := parseCPUTicks("cpu 1 2 3 4 5\n\nctxt 6\n")
	if ticks["cpu"].total == 0 || ticks["__ctxt"].ctx != 6 {
		t.Fatalf("unexpected CPU ticks: %+v", ticks)
	}
}
