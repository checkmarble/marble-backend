package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Configuration struct {
	Database            string
	DbConnectWithSocket bool
	Host                string
	Password            string
	Port                string
	User                string
}

type Database struct {
	pool *pgxpool.Pool
}

type Transaction struct {
	pgx.Tx
}

func (tx *Transaction) Rollback(ctx context.Context) {
	_ = tx.Tx.Rollback(ctx)
}

func (db *Database) Begin(ctx context.Context) (*Transaction, error) {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("db.pool.Begin error: %w", err)
	}
	return &Transaction{
		Tx: tx,
	}, nil
}

func New(conf Configuration) (*Database, error) {
	connectionString := fmt.Sprintf("host=%s user=%s password=%s database=%s sslmode=disable",
		conf.Host,
		conf.User,
		conf.Password,
		conf.Database,
	)
	if !conf.DbConnectWithSocket {
		connectionString = fmt.Sprintf("%s port=%s", connectionString, conf.Port)
	}
	cfg, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}
	cfg.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
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

func NewQueryBuilder() squirrel.StatementBuilderType {
	return squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
}
