package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/docker/docker/client"
)

// ConnectionFallbackWarning explains why a tool had to ignore explicit
// connection parameters and retry against the server's default Docker
// connection. It deliberately contains no certificate contents.
type ConnectionFallbackWarning struct {
	CustomError string `json:"custom_connection_error"`
	Fallback    string `json:"fallback"`
}

// Open resolves the connection and creates a Docker API client. The caller
// owns the returned client and should close it after the operation.
func Open(input ConnectionInput) (*client.Client, error) {
	cli, _, err := NewClient(input)
	return cli, err
}

// openDefault creates a client for Docker's built-in local endpoint without
// consulting any environment variables. It is intentionally separate from
// Open because a fallback must not inherit the broken remote/TLS settings that
// caused the original attempt to fail.
func openDefault() (*client.Client, error) {
	return client.NewClientWithOpts(client.WithHost(defaultHost), client.WithAPIVersionNegotiation())
}

// HasExplicitOptions reports whether the tool request supplied connection
// options. Environment-derived options are intentionally excluded: retrying
// an invalid environment configuration against the same environment would not
// make progress.
func HasExplicitOptions(input ConnectionInput) bool {
	return strings.TrimSpace(input.DockerHost) != "" ||
		strings.TrimSpace(input.DockerCA) != "" ||
		strings.TrimSpace(input.DockerCert) != "" ||
		strings.TrimSpace(input.DockerKey) != "" ||
		strings.TrimSpace(input.DockerServerName) != "" || input.DockerSkipVerify
}

// IsConnectionError identifies failures that are plausibly caused by the
// selected Docker host or TLS material. Docker's client marks transport
// failures explicitly; the net/url checks cover errors raised before the
// Docker client can add that marker.
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	if client.IsErrConnectionFailed(err) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var urlErr *url.Error
	return errors.As(err, &urlErr)
}

// WithFallback executes one Docker operation and retries it once with Docker's
// default local Unix socket when explicit connection options caused a failure. The
// callback must consume/close any response bodies before returning.
func WithFallback(ctx context.Context, input ConnectionInput, operation func(*client.Client) error) (*ConnectionFallbackWarning, error) {
	cli, err := Open(input)
	if err != nil {
		if !HasExplicitOptions(input) {
			return nil, err
		}
		return retryDefault(ctx, err, operation)
	}
	err = operation(cli)
	_ = cli.Close()
	if err == nil || !HasExplicitOptions(input) || !IsConnectionError(err) {
		return nil, err
	}
	return retryDefault(ctx, err, operation)
}

func retryDefault(_ context.Context, customErr error, operation func(*client.Client) error) (*ConnectionFallbackWarning, error) {
	// Intentionally bypass DOCKER_HOST and component environment overrides:
	// this is the fallback for a bad tool-supplied remote host and must use the
	// Docker client's built-in local socket.
	fallback, fallbackErr := openDefault()
	warning := &ConnectionFallbackWarning{
		CustomError: customErr.Error(),
		Fallback:    "自定义 Docker 连接参数失败，已回退到默认连接",
	}
	if fallbackErr != nil {
		return warning, fmt.Errorf("custom Docker connection failed: %v; default connection failed: %w", customErr, fallbackErr)
	}
	err := operation(fallback)
	_ = fallback.Close()
	if err != nil {
		return warning, fmt.Errorf("custom Docker connection failed: %v; default connection failed: %w", customErr, err)
	}
	return warning, nil
}

// Ping verifies that a connection can reach the Docker daemon.
func Ping(ctx context.Context, input ConnectionInput) error {
	cli, err := Open(input)
	if err != nil {
		return err
	}
	defer cli.Close()
	_, err = cli.Ping(ctx)
	return err
}
