package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type transactionOrPool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// implements the DbExecutor from usecasess
type ExecutorPostgres struct {
	databaseShema models.DatabaseSchema
	exec          transactionOrPool
}

func (e ExecutorPostgres) DatabaseSchema() models.DatabaseSchema {
	return e.databaseShema
}

func (e ExecutorPostgres) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return e.exec.Exec(ctx, sql, args...)
}

func (e ExecutorPostgres) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return e.exec.Query(ctx, sql, args...)
}

func (e ExecutorPostgres) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return e.exec.QueryRow(ctx, sql, args...)
}
