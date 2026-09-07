package client

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/docker/client"
)

// ConnectionInput is the connection portion shared by every Docker tool.
// TLS fields accept either Base64-encoded PEM material or a path to a PEM
// file. The server resolves the material and never returns it in tool output.
type ConnectionInput struct {
	DockerHost       string `json:"host,omitempty" jsonschema:"Optional Docker daemon URL."`
	TimeoutSeconds   any    `json:"timeout,omitempty" jsonschema:"Docker request timeout in seconds or duration such as 30s. Defaults to 30s."`
	DockerCA         string `json:"tls_ca,omitempty" jsonschema:"CA PEM material as Base64 text or a file path."`
	DockerCert       string `json:"tls_cert,omitempty" jsonschema:"Client certificate PEM material as Base64 text or a file path."`
	DockerKey        string `json:"tls_key,omitempty" jsonschema:"Client private key PEM material as Base64 text or a file path."`
	DockerServerName string `json:"tls_server_name,omitempty" jsonschema:"Optional TLS server name override."`
	DockerSkipVerify bool   `json:"skip_tls_verify,omitempty" jsonschema:"Skip TLS certificate verification."`
}

// ConnectionConfig is the resolved, non-secret connection configuration.
type ConnectionConfig struct {
	Host       string
	Scheme     string
	CAFile     string
	CertFile   string
	KeyFile    string
	ServerName string
	SkipVerify bool
}

const defaultHost = client.DefaultDockerHost

// ResolveConnection applies the required precedence: tool input, component
// environment, then the Docker client's default local socket.
func ResolveConnection(input ConnectionInput) (ConnectionConfig, error) {
	return resolveConnection(input, true)
}

// resolveConnection can suppress environment fallbacks for draft validation,
// where the result must reflect exactly the connection supplied by the user.
func resolveConnection(input ConnectionInput, inheritEnvironment bool) (ConnectionConfig, error) {
	env := ConnectionInput{
		DockerHost:       firstEnv("DOCKER_MCP_DOCKER_HOST", "DOCKER_HOST"),
		DockerCA:         firstEnv("DOCKER_MCP_DOCKER_CA"),
		DockerCert:       firstEnv("DOCKER_MCP_DOCKER_CERT"),
		DockerKey:        firstEnv("DOCKER_MCP_DOCKER_KEY"),
		DockerServerName: firstEnv("DOCKER_MCP_DOCKER_SERVER_NAME"),
	}
	if inheritEnvironment {
		resolveEnvironmentTLS(&env)
	}

	resolved := input
	if inheritEnvironment {
		if strings.TrimSpace(resolved.DockerHost) == "" {
			resolved.DockerHost = env.DockerHost
		}
		if resolved.DockerCA == "" {
			resolved.DockerCA = env.DockerCA
		}
		if resolved.DockerCert == "" {
			resolved.DockerCert = env.DockerCert
		}
		if resolved.DockerKey == "" {
			resolved.DockerKey = env.DockerKey
		}
		if resolved.DockerServerName == "" {
			resolved.DockerServerName = env.DockerServerName
		}
		if !resolved.DockerSkipVerify {
			resolved.DockerSkipVerify = env.DockerSkipVerify
		}
	}
	if strings.TrimSpace(resolved.DockerHost) == "" {
		return ConnectionConfig{Host: defaultHost, Scheme: "unix"}, nil
	}

	host, scheme, tlsEnabled, err := normalizeHost(resolved.DockerHost, resolved)
	if err != nil {
		return ConnectionConfig{}, err
	}
	if !tlsEnabled && (resolved.DockerCA != "" || resolved.DockerCert != "" || resolved.DockerKey != "" || resolved.DockerServerName != "" || resolved.DockerSkipVerify) {
		return ConnectionConfig{}, fmt.Errorf("TLS configuration requires a tcp or https Docker host")
	}
	if (resolved.DockerCert == "") != (resolved.DockerKey == "") {
		return ConnectionConfig{}, fmt.Errorf("docker client certificate and key must be provided together")
	}
	return ConnectionConfig{
		Host:       host,
		Scheme:     scheme,
		CAFile:     resolved.DockerCA,
		CertFile:   resolved.DockerCert,
		KeyFile:    resolved.DockerKey,
		ServerName: resolved.DockerServerName,
		SkipVerify: resolved.DockerSkipVerify,
	}, nil
}

func resolveEnvironmentTLS(env *ConnectionInput) {
	if env == nil {
		return
	}
	if certDir := os.Getenv("DOCKER_CERT_PATH"); env.DockerCA == "" && certDir != "" {
		env.DockerCA = filepath.Join(certDir, "ca.pem")
		env.DockerCert = filepath.Join(certDir, "cert.pem")
		env.DockerKey = filepath.Join(certDir, "key.pem")
	}
	if skip, ok := lookupBool("DOCKER_MCP_DOCKER_TLS_SKIP_VERIFY"); ok {
		env.DockerSkipVerify = skip
	} else if verify, ok := lookupBool("DOCKER_TLS_VERIFY"); ok {
		env.DockerSkipVerify = !verify
	}
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func lookupBool(name string) (bool, bool) {
	value, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return false, false
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	return parsed, err == nil
}

func normalizeHost(raw string, input ConnectionInput) (host, scheme string, tlsEnabled bool, err error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || (parsed.Host == "" && !strings.EqualFold(parsed.Scheme, "unix")) {
		return "", "", false, fmt.Errorf("invalid Docker host URL")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http":
		if parsed.Path != "" && parsed.Path != "/" {
			return "", "", false, fmt.Errorf("Docker HTTP host paths are not supported")
		}
		return "tcp://" + parsed.Host, "http", false, nil
	case "tcp":
		if parsed.Path != "" && parsed.Path != "/" {
			return "", "", false, fmt.Errorf("Docker TCP host paths are not supported")
		}
		// tcp supports either transport: configure TLS material, Server Name,
		// or skip verification to select HTTPS; otherwise it remains HTTP.
		tlsEnabled := input.DockerCA != "" || input.DockerCert != "" || input.DockerKey != "" || input.DockerServerName != "" || input.DockerSkipVerify
		if tlsEnabled {
			return "tcp://" + parsed.Host, "https", true, nil
		}
		return "tcp://" + parsed.Host, "http", false, nil
	case "https":
		if parsed.Path != "" && parsed.Path != "/" {
			return "", "", false, fmt.Errorf("Docker HTTPS host paths are not supported")
		}
		return "tcp://" + parsed.Host, "https", true, nil
	case "unix":
		if parsed.Host != "" {
			return "", "", false, fmt.Errorf("invalid Docker Unix socket URL")
		}
		return "unix://" + parsed.Path, "unix", false, nil
	default:
		return "", "", false, fmt.Errorf("unsupported Docker host scheme %q", parsed.Scheme)
	}
}

func TLSConfig(config ConnectionConfig) (*tls.Config, error) {
	if config.Scheme != "https" {
		return nil, nil
	}
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         config.ServerName,
		InsecureSkipVerify: config.SkipVerify, //nolint:gosec -- explicitly configured by the operator.
	}
	if config.CAFile != "" {
		pem, err := resolveTLSMaterial(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read Docker CA material: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("Docker CA material does not contain a valid certificate")
		}
		tlsConfig.RootCAs = pool
	}
	if config.CertFile != "" {
		certPEM, err := resolveTLSMaterial(config.CertFile)
		if err != nil {
			return nil, fmt.Errorf("read Docker client certificate material: %w", err)
		}
		keyPEM, err := resolveTLSMaterial(config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("read Docker client key material: %w", err)
		}
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return nil, fmt.Errorf("load Docker client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	return tlsConfig, nil
}

// resolveTLSMaterial treats a TLS value as Base64 text first. This keeps the
// tool contract portable across MCP clients, while the file-path fallback
// preserves compatibility with Docker CLI-style deployments.
func resolveTLSMaterial(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("TLS material is empty")
	}
	encoded := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, value)
	if decoded, err := base64.StdEncoding.DecodeString(encoded); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(encoded); err == nil {
		return decoded, nil
	}
	material, err := os.ReadFile(value)
	if err != nil {
		return nil, fmt.Errorf("value is neither valid Base64 nor a readable file path: %w", err)
	}
	return material, nil
}

// NewClient creates a Docker API client using a resolved connection.
func NewClient(input ConnectionInput) (*client.Client, ConnectionConfig, error) {
	return newClient(input, true)
}

func newClient(input ConnectionInput, inheritEnvironment bool) (*client.Client, ConnectionConfig, error) {
	config, err := resolveConnection(input, inheritEnvironment)
	if err != nil {
		return nil, ConnectionConfig{}, err
	}
	if config.Scheme != "https" {
		cli, err := client.NewClientWithOpts(client.WithHost(config.Host), client.WithAPIVersionNegotiation())
		return cli, config, err
	}
	tlsConfig, err := TLSConfig(config)
	if err != nil {
		return nil, ConnectionConfig{}, err
	}
	httpClient := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}
	cli, err := client.NewClientWithOpts(
		client.WithHost(config.Host),
		client.WithHTTPClient(httpClient),
		client.WithScheme("https"),
		client.WithAPIVersionNegotiation(),
	)
	return cli, config, err
}
