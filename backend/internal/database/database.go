package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ConnectWithRetry(ctx context.Context, databaseURL string, logger *slog.Logger) (*pgxpool.Pool, error) {
	var lastErr error

	for attempt := 1; attempt <= 30; attempt++ {
		pool, err := pgxpool.New(ctx, databaseURL)
		if err == nil {
			pingErr := pool.Ping(ctx)
			if pingErr == nil {
				return pool, nil
			}
			err = pingErr
			pool.Close()
		}

		lastErr = err
		logger.Info("waiting for database", "attempt", attempt, "error", err)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return nil, fmt.Errorf("database is unavailable after retries: %w", lastErr)
}
