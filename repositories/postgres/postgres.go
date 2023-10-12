package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Configuration struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

type Database struct {
	pool *pgxpool.Pool
}

func New(conf Configuration) (*Database, error) {
	connectionString := fmt.Sprintf("host=%s user=%s password=%s database=%s sslmode=disable",
		conf.Host,
		conf.User,
		conf.Password,
		conf.Database,
	)

	pool, err := pgxpool.New(context.Background(), connectionString)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("conn.Ping error: %w", err)
	}

	return &Database{
		pool: pool,
	}, nil
}
