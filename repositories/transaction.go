package repositories

import (
	"context"
	"marble/marble-backend/models"

	"github.com/cockroachdb/errors"
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
	exec          TransactionOrPool
}

type TransactionOrPool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (tx TransactionPostgres) DatabaseSchema() models.DatabaseSchema {
	return tx.databaseShema
}

var ErrIgnoreRoolBackError = errors.New("ignore rollback error")

func IsIsUniqueViolationError(err error) bool {
	var pgxErr *pgconn.PgError
	return errors.As(err, &pgxErr) && pgxErr.Code == pgerrcode.UniqueViolation
}

func (transaction *TransactionPostgres) SqlExec(query string, args ...any) (rowsAffected int64, err error) {

	tag, err := transaction.exec.Exec(transaction.ctx, query, args...)
	if err != nil {
		return 0, errors.Wrap(err, "Error executing sql query")
	}
	return tag.RowsAffected(), nil
}

type AnyBuilder interface {
	ToSql() (string, []interface{}, error)
}

func (transaction *TransactionPostgres) ExecBuilder(builder AnyBuilder) (rowsAffected int64, err error) {
	query, args, err := builder.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "Error building sql query")
	}

	return transaction.SqlExec(query, args...)
}
