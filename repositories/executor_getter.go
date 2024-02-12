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

func NewExecutorGetter(pool *pgxpool.Pool) ExecutorGetter {
	return ExecutorGetter{
		connectionPool: pool,
	}
}

func (g *ExecutorGetter) Transaction(
	ctx context.Context,
	databaseSchema models.DatabaseSchema,
	fn func(exec *ExecutorPostgres) error,
) error {
	err := pgx.BeginFunc(ctx, g.connectionPool, func(tx pgx.Tx) error {
		return fn(&ExecutorPostgres{
			databaseShema: databaseSchema,
			exec:          tx,
		})
	})

	// helper: The callback can return ErrIgnoreRollBackError
	// to explicitly specify that the error should be ignored.
	if errors.Is(err, ErrIgnoreRollBackError) {
		return nil
	}
	return errors.Wrap(err, "Error executing transaction")
}

func (g *ExecutorGetter) GetExecutor(databaseSchema models.DatabaseSchema) *ExecutorPostgres {
	return &ExecutorPostgres{
		databaseShema: databaseSchema,
		exec:          g.connectionPool,
	}
}

func (g *ExecutorGetter) ifNil(exec *ExecutorPostgres) *ExecutorPostgres {
	if exec == nil {
		exec = &ExecutorPostgres{
			databaseShema: models.DATABASE_MARBLE_SCHEMA,
			exec:          g.connectionPool,
		}
	}
	return exec
}
