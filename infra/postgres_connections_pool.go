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

type PGConfig struct {
	ConnectionString    string
	Database            string
	DbConnectWithSocket bool
	Hostname            string
	Password            string
	Port                string
	User                string
}

func (config PGConfig) GetConnectionString() string {
	if config.ConnectionString != "" {
		return config.ConnectionString
	}
	connectionString := fmt.Sprintf("host=%s user=%s password=%s database=%s sslmode=disable",
		config.Hostname, config.User, config.Password, config.Database)
	if !config.DbConnectWithSocket {
		// Cloud Run connects to the DB through a proxy and a unix socket, so we don't need need to specify the port
		// but we do when running locally
		connectionString = fmt.Sprintf("%s port=%s", connectionString, config.Port)
	}
	return connectionString
}

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
