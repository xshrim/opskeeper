package host

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
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

func TestKnownHostsIsOptional(t *testing.T) {
	callback, cleanup, err := knownHostsCallback("")
	if err != nil || callback == nil {
		t.Fatalf("empty known_hosts should be accepted: callback=%v err=%v", callback != nil, err)
	}
	if err := callback("unknown.example:22", nil, nil); err != nil {
		t.Fatalf("empty known_hosts should allow an unknown host key: %v", err)
	}
	cleanup()
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

func TestParseBatchFiles(t *testing.T) {
	raw := []byte(batchFileBegin + "/proc/a\nvalue-a\n" + batchFileEnd + "\n" + batchFileBegin + "/proc/b\nvalue-b\n" + batchFileEnd + "\n")
	files := parseBatchFiles(raw)
	if string(files["/proc/a"]) != "value-a" || string(files["/proc/b"]) != "value-b" {
		t.Fatalf("unexpected batch files: %#v", files)
	}
}

func TestParsePSProcesses(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	raw := []byte("123 1 S 0 4 1024 4096 30 2.5 00:01:02 worker /usr/bin/worker --token secret\n")
	processes := parsePSProcesses(raw, now, []byte("root:x:0:0:root:/root:/bin/sh\n"))
	if len(processes) != 1 {
		t.Fatalf("process count = %d", len(processes))
	}
	process := processes[0]
	if process.PID != 123 || process.User != "root" || process.RSSBytes != 1024*1024 || process.CPUUsagePercent != 2.5 {
		t.Fatalf("unexpected process: %+v", process)
	}
	if process.CPUTimeSeconds != 62 {
		t.Fatalf("unexpected CPU time: %v", process.CPUTimeSeconds)
	}
	if strings.Contains(process.CommandLine, "secret") {
		t.Fatalf("command line was not redacted: %q", process.CommandLine)
	}
}

func TestParseProcessExtras(t *testing.T) {
	extras := parseProcessExtras([]byte(processExtraMarker + "\t123\t/usr/bin/worker\t/work\t/\t7\t11\t13\n"))
	value, ok := extras[123]
	if !ok || value.Executable != "/usr/bin/worker" || value.CWD != "/work" || value.FileDescriptors != 7 || value.ReadBytes != 11 || value.WriteBytes != 13 {
		t.Fatalf("unexpected process extras: %#v", extras)
	}
}

func TestParseRemoteSnapshotSections(t *testing.T) {
	raw := []byte(remoteProcessBegin + "\nR\nS\n" + remoteProcessEnd + "\n" + remoteFSBegin + "\n4096 10 3 4\n" + remoteFSEnd + "\n")
	processRaw, ok := parseSection(raw, remoteProcessBegin, remoteProcessEnd)
	if !ok || string(processRaw) != "R\nS" {
		t.Fatalf("unexpected process section: %q, %v", processRaw, ok)
	}
	processes, ok := parseProcessSummary(string(processRaw))
	if !ok || processes.Total != 2 || processes.Running != 1 || processes.Sleeping != 1 {
		t.Fatalf("unexpected process summary: %+v, %v", processes, ok)
	}
	fsRaw, ok := parseSection(raw, remoteFSBegin, remoteFSEnd)
	fs, fsOK := parseFSStat(string(fsRaw))
	if !ok || !fsOK || fs.Total != 40960 || fs.Free != 12288 || fs.Available != 16384 {
		t.Fatalf("unexpected filesystem section: %+v, %v, %v", fs, ok, fsOK)
	}
}
