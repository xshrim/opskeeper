package client

import (
	"context"

	"github.com/docker/docker/client"
)

// Open resolves the connection and creates a Docker API client. The caller
// owns the returned client and should close it after the operation.
func Open(input ConnectionInput) (*client.Client, error) {
	cli, _, err := NewClient(input)
	return cli, err
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
