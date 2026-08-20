package client

import (
	"crypto/tls"
	"crypto/x509"
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
// Certificate paths are read by the server process and are never returned in
// tool output.
type ConnectionInput struct {
	DockerHost       string `json:"docker_host,omitempty" jsonschema:"Optional Docker daemon URL. Tool input takes precedence over the environment."`
	DockerCA         string `json:"docker_ca,omitempty" jsonschema:"CA PEM file for an HTTPS Docker daemon."`
	DockerCert       string `json:"docker_cert,omitempty" jsonschema:"Client certificate PEM file for mutual TLS."`
	DockerKey        string `json:"docker_key,omitempty" jsonschema:"Client private key PEM file for mutual TLS."`
	DockerServerName string `json:"docker_server_name,omitempty" jsonschema:"Optional TLS server name override."`
	DockerSkipVerify bool   `json:"docker_skip_tls_verify,omitempty" jsonschema:"Skip TLS certificate verification. Defaults to false; use only for explicitly trusted development endpoints."`
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
	env := ConnectionInput{
		DockerHost:       firstEnv("DOCKER_MCP_DOCKER_HOST", "DOCKER_HOST"),
		DockerCA:         firstEnv("DOCKER_MCP_DOCKER_CA"),
		DockerCert:       firstEnv("DOCKER_MCP_DOCKER_CERT"),
		DockerKey:        firstEnv("DOCKER_MCP_DOCKER_KEY"),
		DockerServerName: firstEnv("DOCKER_MCP_DOCKER_SERVER_NAME"),
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

	resolved := input
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
	// A bool input has a zero-value default of false. Preserve the existing
	// environment fallback for deployments that configure TLS globally; a
	// true tool value always wins, while false cannot express explicit
	// override because omitted and false are identical in JSON.
	if !resolved.DockerSkipVerify {
		resolved.DockerSkipVerify = env.DockerSkipVerify
	}
	if strings.TrimSpace(resolved.DockerHost) == "" {
		return ConnectionConfig{Host: defaultHost, Scheme: "unix"}, nil
	}

	host, scheme, tlsEnabled, err := normalizeHost(resolved.DockerHost, resolved)
	if err != nil {
		return ConnectionConfig{}, err
	}
	if !tlsEnabled && (resolved.DockerCA != "" || resolved.DockerCert != "" || resolved.DockerKey != "" || resolved.DockerServerName != "") {
		return ConnectionConfig{}, fmt.Errorf("TLS files or server name require an https Docker host")
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
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
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
		// A tcp URL becomes TLS when certificate material or the explicit
		// skip-verification switch is supplied, matching Docker CLI practice.
		tlsEnabled := input.DockerCA != "" || input.DockerCert != "" || input.DockerKey != "" || input.DockerSkipVerify
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
		pem, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read Docker CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("Docker CA file does not contain a valid certificate")
		}
		tlsConfig.RootCAs = pool
	}
	if config.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load Docker client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	return tlsConfig, nil
}

// NewClient creates a Docker API client using a resolved connection.
func NewClient(input ConnectionInput) (*client.Client, ConnectionConfig, error) {
	config, err := ResolveConnection(input)
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
