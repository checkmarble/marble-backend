package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/trace"
)

const (
	MAX_CONNECTIONS          = 40
	MAX_CONNECTION_IDLE_TIME = 5 * time.Minute
)

func NewPostgresConnectionPool(ctx context.Context, connectionString string, tp trace.TracerProvider) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}
	ops := []otelpgx.Option{}
	if tp != nil {
		ops = append(ops, otelpgx.WithTracerProvider(tp))
	}
	cfg.ConnConfig.Tracer = otelpgx.NewTracer(ops...)
	cfg.MaxConns = MAX_CONNECTIONS
	cfg.MaxConnIdleTime = MAX_CONNECTION_IDLE_TIME

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return pool, retry.Do(
		func() error {
			if err := pool.Ping(ctx); err != nil {
				return fmt.Errorf("NewPostgresConnectionPool.Ping error: %w", err)
			}
			return err
		},
		retry.Attempts(3),
		retry.LastErrorOnly(true),
	)
}
