package repositories

import (
	"context"
	"errors"
	"marble/marble-backend/models"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Opaque type, down-casted to TransactionPostgres in by repositories
type Transaction interface {
	DatabaseSchema() models.DatabaseSchema
}

type TransactionPostgres struct {
	databaseShema models.DatabaseSchema
	ctx           context.Context
	exec          TransactionOrPool // Optional, used when no transaction is provided
}

type TransactionOrPool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (tx TransactionPostgres) DatabaseSchema() models.DatabaseSchema {
	return tx.databaseShema
}

// helper
func (transaction *TransactionPostgres) Exec(sql string, arguments ...any) (pgconn.CommandTag, error) {
	return transaction.exec.Exec(transaction.ctx, sql, arguments...)
}

// helper
func (transaction *TransactionPostgres) Query(sql string, arguments ...any) (pgx.Rows, error) {
	return transaction.exec.Query(transaction.ctx, sql, arguments...)
}

// helper
func (transaction *TransactionPostgres) QueryRow(sql string, arguments ...any) pgx.Row {
	return transaction.exec.QueryRow(transaction.ctx, sql, arguments...)
}

var ErrIgnoreRoolBackError = errors.New("ignore rollback error")

func IsIsUniqueViolationError(err error) bool {
	var pgxErr *pgconn.PgError
	return errors.As(err, &pgxErr) && pgxErr.Code == pgerrcode.UniqueViolation
}
