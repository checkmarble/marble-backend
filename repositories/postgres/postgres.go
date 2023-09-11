package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Configuration struct {
	Hostname string
	Port     string
	User     string
	Password string
	Database string
}

type databasePool interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

type Database struct {
	pool databasePool
}

func New(conf Configuration) (*Database, error) {
	connString := fmt.Sprintf("host=%s user=%s password=%s database=%s sslmode=disable", conf.Hostname, conf.User, conf.Password, conf.Database)
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New error: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("pool.Ping error: %w", err)
	}

	return &Database{
		pool: pool,
	}, nil
}
