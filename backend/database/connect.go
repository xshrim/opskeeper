package database

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const retryInterval = 2 * time.Second

// OpenPool waits for PostgreSQL so service startup does not depend on container order.
func OpenPool(ctx context.Context, url string, logger *slog.Logger) (*pgxpool.Pool, error) {
	for {
		pool, err := pgxpool.New(ctx, url)
		if err == nil {
			err = pool.Ping(ctx)
		}
		if err == nil {
			return pool, nil
		}
		if pool != nil {
			pool.Close()
		}
		logger.Warn("PostgreSQL is unavailable; retrying", "kind", "dependency", "error_type", "postgres-connect", "error", err)
		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
