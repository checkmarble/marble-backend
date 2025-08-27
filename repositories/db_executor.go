package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// //////////////////////////////////
// Generic db executor (tx or pool)
// //////////////////////////////////

type pgxTxOrPool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

type TransactionOrPool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (Transaction, error)
}

type PgExecutor struct {
	databaseSchema models.DatabaseSchema
	exec           pgxTxOrPool
}

func (e PgExecutor) DatabaseSchema() models.DatabaseSchema {
	return e.databaseSchema
}

func (e PgExecutor) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tag, err := injectDbSessionConfig(ctx, e.exec); err != nil {
		return tag, err
	}

	return e.exec.Exec(ctx, sql, args...)
}

func (e PgExecutor) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if _, err := injectDbSessionConfig(ctx, e.exec); err != nil {
		return nil, err
	}

	return e.exec.Query(ctx, sql, args...)
}

func (e PgExecutor) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if _, err := injectDbSessionConfig(ctx, e.exec); err != nil {
		return errorRow{err}
	}

	return e.exec.QueryRow(ctx, sql, args...)
}

func (e PgExecutor) Begin(ctx context.Context) (Transaction, error) {
	tx, err := e.exec.Begin(ctx)
	if err != nil {
		return PgTx{}, errors.Wrap(err, "Error beginning transaction")
	}
	return PgTx{
		databaseSchema: e.databaseSchema,
		tx:             tx,
	}, nil
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

func (t PgTx) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t PgTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

func (t PgTx) Begin(ctx context.Context) (Transaction, error) {
	tx, err := t.tx.Begin(ctx)
	if err != nil {
		return PgTx{}, errors.Wrap(err, "Error beginning transaction")
	}
	return PgTx{
		databaseSchema: t.databaseSchema,
		tx:             tx,
	}, nil
}
