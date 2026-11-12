package database

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

func TestOpenPoolStopsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := OpenPool(ctx, "postgres://127.0.0.1:1/opskeeper", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != context.Canceled {
		t.Fatalf("OpenPool() error = %v, want context.Canceled", err)
	}
}
