package repositories

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"

	"github.com/cockroachdb/errors"
)

// Opaque type, down-casted to TransactionPostgres in by repositories
type Transaction_deprec interface {
	DatabaseSchema() models.DatabaseSchema
}

type TransactionPostgres_deprec struct {
	databaseShema models.DatabaseSchema
	exec          transactionOrPool
}

func (tx TransactionPostgres_deprec) DatabaseSchema() models.DatabaseSchema {
	return tx.databaseShema
}

func (transaction *TransactionPostgres_deprec) SqlExec(ctx context.Context, query string, args ...any) (rowsAffected int64, err error) {

	tag, err := transaction.exec.Exec(ctx, query, args...)
	if err != nil {
		return 0, errors.Wrap(err, fmt.Sprintf("error executing sql query: %s", query))
	}
	return tag.RowsAffected(), nil
}

type AnyBuilder interface {
	ToSql() (string, []interface{}, error)
}

func (transaction *TransactionPostgres_deprec) ExecBuilder(ctx context.Context, builder AnyBuilder) (rowsAffected int64, err error) {
	query, args, err := builder.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "can't build sql query")
	}

	return transaction.SqlExec(ctx, query, args...)
}
