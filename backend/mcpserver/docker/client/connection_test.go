package client

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestResolveConnectionTCPSelectsTLSOnlyWhenConfigured(t *testing.T) {
	t.Setenv("DOCKER_MCP_DOCKER_TLS_SKIP_VERIFY", "")
	plain, err := ResolveConnection(ConnectionInput{DockerHost: "tcp://docker.example:2376"})
	if err != nil {
		t.Fatal(err)
	}
	if plain.Scheme != "http" {
		t.Fatalf("plain TCP scheme = %q, want http", plain.Scheme)
	}
	tlsConnection, err := ResolveConnection(ConnectionInput{DockerHost: "tcp://docker.example:2376", DockerSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	if tlsConnection.Scheme != "https" || !tlsConnection.SkipVerify {
		t.Fatalf("TLS TCP connection = %+v", tlsConnection)
	}
}

func TestResolveConnectionRejectsTLSConfigurationForHTTPAndUnix(t *testing.T) {
	for _, host := range []string{"http://docker.example:2375", "unix:///var/run/docker.sock"} {
		if _, err := ResolveConnection(ConnectionInput{DockerHost: host, DockerSkipVerify: true}); err == nil {
			t.Fatalf("expected %s TLS configuration to fail", host)
		}
	}
}

func TestDraftConnectionDoesNotInheritEnvironmentTLS(t *testing.T) {
	t.Setenv("DOCKER_MCP_DOCKER_CA", "/unrelated/ca.pem")
	t.Setenv("DOCKER_MCP_DOCKER_TLS_SKIP_VERIFY", "true")
	config, err := resolveConnection(ConnectionInput{DockerHost: "tcp://docker.example:2376"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if config.Scheme != "http" || config.CAFile != "" || config.SkipVerify {
		t.Fatalf("draft connection inherited environment TLS: %+v", config)
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

func TestResolveTLSMaterialPrefersBase64AndFallsBackToFile(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("base64 material"))
	got, err := resolveTLSMaterial(encoded)
	if err != nil {
		t.Fatalf("decode Base64 material: %v", err)
	}
	if string(got) != "base64 material" {
		t.Fatalf("decoded material = %q", got)
	}
	got, err = resolveTLSMaterial("Y 2 E =\n")
	if err != nil || string(got) != "ca" {
		t.Fatalf("decode wrapped Base64 material: %q, %v", got, err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(path, []byte("file material"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = resolveTLSMaterial(path)
	if err != nil {
		t.Fatalf("read path material: %v", err)
	}
	if string(got) != "file material" {
		t.Fatalf("file material = %q", got)
	}

	if _, err := resolveTLSMaterial(filepath.Join(dir, "missing.pem")); err == nil {
		t.Fatal("expected missing path to return an error")
	}
}

func TestTLSConfigAcceptsBase64AndFileTLSMaterial(t *testing.T) {
	certPEM, keyPEM := testCertificateMaterial(t)
	caPEM := certPEM
	base64Config := ConnectionConfig{
		Scheme:   "https",
		CAFile:   base64.StdEncoding.EncodeToString(caPEM),
		CertFile: base64.StdEncoding.EncodeToString(certPEM),
		KeyFile:  base64.StdEncoding.EncodeToString(keyPEM),
	}
	tlsConfig, err := TLSConfig(base64Config)
	if err != nil {
		t.Fatalf("load Base64 TLS material: %v", err)
	}
	if tlsConfig.RootCAs == nil || len(tlsConfig.Certificates) != 1 {
		t.Fatalf("unexpected Base64 TLS config: %+v", tlsConfig)
	}

	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	for path, material := range map[string][]byte{caPath: caPEM, certPath: certPEM, keyPath: keyPEM} {
		if err := os.WriteFile(path, material, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fileConfig, err := TLSConfig(ConnectionConfig{Scheme: "https", CAFile: caPath, CertFile: certPath, KeyFile: keyPath})
	if err != nil {
		t.Fatalf("load file TLS material: %v", err)
	}
	if fileConfig.RootCAs == nil || len(fileConfig.Certificates) != 1 {
		t.Fatalf("unexpected file TLS config: %+v", fileConfig)
	}
}

func testCertificateMaterial(t *testing.T) ([]byte, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return certPEM, keyPEM
}
