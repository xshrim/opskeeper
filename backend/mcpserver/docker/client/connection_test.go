package client

import (
	"errors"
	"testing"

	dockerapi "github.com/docker/docker/client"
)

func TestWithFallbackRetriesExplicitConnectionConstructionErrors(t *testing.T) {
	t.Setenv("DOCKER_MCP_DOCKER_HOST", "tcp://environment.example:2375")
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_MCP_DOCKER_CA", "/path/that/does/not/exist/ca.pem")
	var calls int
	warning, err := WithFallback(t.Context(), ConnectionInput{DockerHost: "not-a-docker-url"}, func(_ *dockerapi.Client) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("fallback returned error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("operation calls = %d, want 1 after default fallback", calls)
	}
	if warning == nil || warning.CustomError == "" {
		t.Fatalf("missing fallback warning: %+v", warning)
	}
}

func TestWithFallbackRetriesUnreachableRemoteHostWithDefaultSocket(t *testing.T) {
	t.Setenv("DOCKER_MCP_DOCKER_HOST", "")
	t.Setenv("DOCKER_HOST", "")
	var hosts []string
	warning, err := WithFallback(t.Context(), ConnectionInput{DockerHost: "tcp://127.0.0.1:1"}, func(cli *dockerapi.Client) error {
		hosts = append(hosts, cli.DaemonHost())
		if cli.DaemonHost() != defaultHost {
			return dockerapi.ErrorConnectionFailed(cli.DaemonHost())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("fallback returned error: %v", err)
	}
	if len(hosts) != 2 || hosts[0] != "tcp://127.0.0.1:1" || hosts[1] != defaultHost {
		t.Fatalf("connection hosts = %#v, want remote host followed by %q", hosts, defaultHost)
	}
	if warning == nil || warning.CustomError == "" {
		t.Fatalf("missing fallback warning: %+v", warning)
	}
}

func TestWithFallbackDoesNotRetryBusinessErrors(t *testing.T) {
	t.Setenv("DOCKER_MCP_DOCKER_HOST", "")
	t.Setenv("DOCKER_HOST", "")
	var calls int
	warning, err := WithFallback(t.Context(), ConnectionInput{DockerHost: "unix:///var/run/docker.sock"}, func(_ *dockerapi.Client) error {
		calls++
		return errors.New("container not found")
	})
	if err == nil || err.Error() != "container not found" {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 || warning != nil {
		t.Fatalf("business error was retried: calls=%d warning=%+v", calls, warning)
	}
}

func TestResolveConnectionPrecedence(t *testing.T) {
	t.Setenv("DOCKER_MCP_DOCKER_HOST", "http://env.example:2375")
	t.Setenv("DOCKER_MCP_DOCKER_TLS_SKIP_VERIFY", "false")
	got, err := ResolveConnection(ConnectionInput{DockerHost: "http://tool.example:2375"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "tcp://tool.example:2375" || got.Scheme != "http" {
		t.Fatalf("tool connection did not win: %+v", got)
	}

	got, err = ResolveConnection(ConnectionInput{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "tcp://env.example:2375" || got.Scheme != "http" {
		t.Fatalf("environment connection was not selected: %+v", got)
	}
}

func TestResolveConnectionDefaultsToSocket(t *testing.T) {
	t.Setenv("DOCKER_MCP_DOCKER_HOST", "")
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_MCP_DOCKER_TLS_SKIP_VERIFY", "")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	got, err := ResolveConnection(ConnectionInput{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != defaultHost || got.Scheme != "unix" {
		t.Fatalf("unexpected default connection: %+v", got)
	}
	if got.SkipVerify {
		t.Fatal("skip TLS verification must default to false")
	}
}

func TestResolveConnectionTLSAndSkipVerify(t *testing.T) {
	t.Setenv("DOCKER_MCP_DOCKER_HOST", "https://docker.example:2376")
	t.Setenv("DOCKER_MCP_DOCKER_TLS_SKIP_VERIFY", "")
	got, err := ResolveConnection(ConnectionInput{DockerSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Scheme != "https" || !got.SkipVerify {
		t.Fatalf("unexpected TLS connection: %+v", got)
	}
}

func TestResolveConnectionTLSAndSkipVerifyFromEnvironment(t *testing.T) {
	t.Setenv("DOCKER_MCP_DOCKER_HOST", "https://docker.example:2376")
	t.Setenv("DOCKER_MCP_DOCKER_TLS_SKIP_VERIFY", "true")
	got, err := ResolveConnection(ConnectionInput{})
	if err != nil {
		t.Fatal(err)
	}
	if !got.SkipVerify {
		t.Fatal("expected environment TLS skip verification to be retained")
	}
}

func TestResolveConnectionRejectsPartialClientCertificate(t *testing.T) {
	t.Setenv("DOCKER_MCP_DOCKER_HOST", "https://docker.example:2376")
	_, err := ResolveConnection(ConnectionInput{DockerCert: "/tmp/cert.pem"})
	if err == nil {
		t.Fatal("expected partial certificate configuration to fail")
	}
}
