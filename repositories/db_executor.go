package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// //////////////////////////////////
// Generic db executor (tx or pool)
// //////////////////////////////////
type TransactionOrPool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type PgExecutor struct {
	databaseSchema models.DatabaseSchema
	exec           TransactionOrPool
}

func (e PgExecutor) DatabaseSchema() models.DatabaseSchema {
	return e.databaseSchema
}

func (e PgExecutor) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return e.exec.Exec(ctx, sql, args...)
}

func (e PgExecutor) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return e.exec.Query(ctx, sql, args...)
}

func (e PgExecutor) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return e.exec.QueryRow(ctx, sql, args...)
}

////////////////////////////////////
// Transaction
////////////////////////////////////

type PgTx struct {
	databaseSchema models.DatabaseSchema
	tx             pgx.Tx
}

func (t PgTx) DatabaseSchema() models.DatabaseSchema {
	return t.databaseSchema
}

func (t PgTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return t.tx.Exec(ctx, sql, args...)
}

func (t PgTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return t.tx.Query(ctx, sql, args...)
}

func (t PgTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return t.tx.QueryRow(ctx, sql, args...)
}

func (t PgTx) RawTx() pgx.Tx {
	return t.tx
}
