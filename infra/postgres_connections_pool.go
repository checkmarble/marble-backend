package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/trace"
)

const (
	DEFAULT_MAX_CONNECTIONS  = 40 // TODO: make this a configurable value
	MAX_CONNECTION_IDLE_TIME = 5 * time.Minute
)

type ClientDbConfig struct {
	ConnectionString string `json:"connection_string"`
	MaxConns         int    `json:"max_conns"`
	SchemaName       string `json:"schema_name"`
}

func NewPostgresConnectionPool(
	ctx context.Context,
	connectionString string,
	tp trace.TracerProvider,
	maxConnections int,
) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}
	ops := []otelpgx.Option{}
	if tp != nil {
		ops = append(ops, otelpgx.WithTracerProvider(tp))
	}
	cfg.ConnConfig.Tracer = otelpgx.NewTracer(ops...)
	cfg.MaxConns = int32(maxConnections)
	if cfg.MaxConns == 0 {
		cfg.MaxConns = DEFAULT_MAX_CONNECTIONS
	}
	cfg.MaxConnIdleTime = MAX_CONNECTION_IDLE_TIME

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
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

func ParseClientDbConfig(filename string) (map[string]ClientDbConfig, error) {
	if filename == "" {
		return nil, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	clientDbConfigs := make(map[string]ClientDbConfig)
	if err := json.NewDecoder(file).Decode(&clientDbConfigs); err != nil {
		return nil, err
	}
	return clientDbConfigs, nil
}
