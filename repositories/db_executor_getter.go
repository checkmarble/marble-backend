package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"

	"github.com/cockroachdb/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExecutorGetter struct {
	connectionPool *pgxpool.Pool
}

type databaseSchemaGetter interface {
	DatabaseSchema() models.DatabaseSchema
}

type Executor interface {
	TransactionOrPool
	databaseSchemaGetter
}

type Transaction interface {
	databaseSchemaGetter
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	RawTx() pgx.Tx
}

func NewExecutorGetter(pool *pgxpool.Pool) ExecutorGetter {
	return ExecutorGetter{
		connectionPool: pool,
	}
}

func (g ExecutorGetter) Transaction(
	ctx context.Context,
	databaseSchema models.DatabaseSchema,
	fn func(exec Transaction) error,
) error {
	err := pgx.BeginFunc(ctx, g.connectionPool, func(tx pgx.Tx) error {
		return fn(&PgTx{
			databaseSchema: databaseSchema,
			tx:             tx,
		})
	})

	// helper: The callback can return ErrIgnoreRollBackError
	// to explicitly specify that the error should be ignored.
	if errors.Is(err, models.ErrIgnoreRollBackError) {
		return nil
	}
	return errors.Wrap(err, "Error executing transaction")
}

func (g ExecutorGetter) GetExecutor(databaseSchema models.DatabaseSchema) Executor {
	return &PgExecutor{
		databaseSchema: databaseSchema,
		exec:           g.connectionPool,
	}
}

func validateClientDbExecutor(exec databaseSchemaGetter) error {
	if exec == nil {
		return errors.New("Cannot use nil executor for client database")
	}
	if exec.DatabaseSchema().SchemaType != models.DATABASE_SCHEMA_TYPE_CLIENT {
		return errors.New("Cannot use marble db executor to query client database")
	}
	return nil
}

func validateMarbleDbExecutor(exec databaseSchemaGetter) error {
	if exec == nil {
		return errors.New("Cannot use nil executor for marble database")
	}
	if exec.DatabaseSchema().SchemaType != models.DATABASE_SCHEMA_TYPE_MARBLE {
		return errors.New("Cannot use client db executor to query marble database")
	}
	return nil
}
