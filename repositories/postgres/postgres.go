package postgres

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

func New(pool *pgxpool.Pool) *Database {
	return &Database{
		pool: pool,
	}
}

func NewQueryBuilder() squirrel.StatementBuilderType {
	return squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
}
