//go:build integration

package host

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestConfiguredHostMetrics(t *testing.T) {
	host := os.Getenv("OPSK_TEST_HOST_ADDRESS")
	username := os.Getenv("OPSK_TEST_HOST_USERNAME")
	password := os.Getenv("OPSK_TEST_HOST_PASSWORD")
	if host == "" || username == "" || password == "" {
		t.Skip("OPSK_TEST_HOST_ADDRESS, OPSK_TEST_HOST_USERNAME, and OPSK_TEST_HOST_PASSWORD are required")
	}

	port := 22
	if value := os.Getenv("OPSK_TEST_HOST_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			t.Fatalf("invalid OPSK_TEST_HOST_PORT: %v", err)
		}
		port = parsed
	}
	authMethod := os.Getenv("OPSK_TEST_HOST_AUTH_METHOD")
	if authMethod == "" {
		authMethod = "password"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	output, err := Metrics(ctx, MetricsInput{ConnectionInput: ConnectionInput{
		Host:       host,
		Port:       port,
		Username:   username,
		AuthMethod: authMethod,
		Password:   password,
	}, SampleSeconds: 10})
	if err != nil {
		t.Fatal(err)
	}
	if output.Partial {
		t.Fatalf("host metrics are partial: unavailable=%v", output.Unavailable)
	}
	if len(output.Metrics.CPU.PerCPU) == 0 {
		t.Fatal("CPU metrics are empty")
	}
	if output.Metrics.Memory.TotalBytes == 0 {
		t.Fatal("memory metrics are empty")
	}
	if len(output.Metrics.Disk) == 0 {
		t.Fatal("disk metrics are empty")
	}
	if len(output.Metrics.Network) == 0 {
		t.Fatal("network metrics are empty")
	}
	if len(output.Metrics.Filesystem) == 0 {
		t.Fatal("filesystem metrics are empty")
	}
}

func TestConfiguredHostInfoAndProcesses(t *testing.T) {
	host := os.Getenv("OPSK_TEST_HOST_ADDRESS")
	username := os.Getenv("OPSK_TEST_HOST_USERNAME")
	password := os.Getenv("OPSK_TEST_HOST_PASSWORD")
	if host == "" || username == "" || password == "" {
		t.Skip("OPSK_TEST_HOST_ADDRESS, OPSK_TEST_HOST_USERNAME, and OPSK_TEST_HOST_PASSWORD are required")
	}
	port := 22
	if value := os.Getenv("OPSK_TEST_HOST_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			t.Fatalf("invalid OPSK_TEST_HOST_PORT: %v", err)
		}
		port = parsed
	}
	authMethod := os.Getenv("OPSK_TEST_HOST_AUTH_METHOD")
	if authMethod == "" {
		authMethod = "password"
	}
	connection := ConnectionInput{Host: host, Port: port, Username: username, AuthMethod: authMethod, Password: password}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	started := time.Now()
	info, err := Info(ctx, InfoInput{ConnectionInput: connection})
	t.Logf("info duration=%s", time.Since(started))
	if err != nil {
		t.Fatal(err)
	}
	if info.Hostname == "" || info.CPUs == 0 || info.MemoryBytes == 0 {
		t.Fatalf("incomplete host info: %+v", info)
	}

	started = time.Now()
	output, err := Processes(ctx, ProcessesInput{ConnectionInput: connection, Keyword: "sshd", Limit: 5})
	t.Logf("processes duration=%s count=%d unavailable=%v", time.Since(started), len(output.Processes), output.Unavailable)
	if err != nil {
		t.Fatal(err)
	}
	if output.Partial || len(output.Unavailable) > 0 {
		t.Fatalf("process result is partial: %+v", output)
	}
}
