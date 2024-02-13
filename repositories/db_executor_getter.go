package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"

	"github.com/cockroachdb/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExecutorGetter struct {
	connectionPool *pgxpool.Pool
}

type databaseSchemaGetter interface {
	DatabaseSchema() models.DatabaseSchema
}

type Executor interface {
	transactionOrPool
	databaseSchemaGetter
}

func NewExecutorGetter(pool *pgxpool.Pool) ExecutorGetter {
	return ExecutorGetter{
		connectionPool: pool,
	}
}

func (g ExecutorGetter) Transaction(
	ctx context.Context,
	databaseSchema models.DatabaseSchema,
	fn func(exec Executor) error,
) error {
	err := pgx.BeginFunc(ctx, g.connectionPool, func(tx pgx.Tx) error {
		return fn(&ExecutorPostgres{
			databaseShema: databaseSchema,
			exec:          tx,
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
	return &ExecutorPostgres{
		databaseShema: databaseSchema,
		exec:          g.connectionPool,
	}
}

func (g ExecutorGetter) ifNil(exec Executor) Executor {
	if exec == nil {
		exec = &ExecutorPostgres{
			databaseShema: models.DATABASE_MARBLE_SCHEMA,
			exec:          g.connectionPool,
		}
	}
	return exec
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
