package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	MAX_CONNECTIONS          = 40
	MAX_CONNECTION_IDLE_TIME = 5 * time.Minute
)

func NewPostgresConnectionPool(ctx context.Context, connectionString string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}
	cfg.ConnConfig.Tracer = otelpgx.NewTracer()
	cfg.MaxConns = MAX_CONNECTIONS
	cfg.MaxConnIdleTime = MAX_CONNECTION_IDLE_TIME

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("NewPostgresConnectionPool.Ping error: %w", err)
	}

	return pool, nil
}
