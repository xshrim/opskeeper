package client

import "testing"

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
