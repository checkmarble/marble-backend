package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// implements the DbExecutor from usecasess
type ExecutorPostgres struct {
	databaseShema models.DatabaseSchema
	exec          TransactionOrPool
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

func validateClientDbExecutor(exec *ExecutorPostgres) (*ExecutorPostgres, error) {
	if exec == nil {
		return nil, errors.New("Cannot adapt nil executor for client database")
	}

	if exec.DatabaseSchema().SchemaType != models.DATABASE_SCHEMA_TYPE_CLIENT {
		return nil, errors.New("Can only handle transactions in Client database.")
	}
	return exec, nil
}
