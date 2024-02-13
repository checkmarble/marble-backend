package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"

	"github.com/cockroachdb/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DatabaseConnectionPoolRepository_deprec interface {
	DatabaseConnectionPool(ctx context.Context, connection models.PostgresConnection) (*pgxpool.Pool, error)
}

type TransactionFactoryPosgresql_deprec struct {
	databaseConnectionPoolRepository DatabaseConnectionPoolRepository_deprec
	marbleConnectionPool             *pgxpool.Pool
}

func NewTransactionFactoryPosgresql_deprec(
	databaseConnectionPoolRepository DatabaseConnectionPoolRepository_deprec,
	marbleConnectionPool *pgxpool.Pool,
) TransactionFactoryPosgresql_deprec {
	return TransactionFactoryPosgresql_deprec{
		databaseConnectionPoolRepository: databaseConnectionPoolRepository,
		marbleConnectionPool:             marbleConnectionPool,
	}
}

func (factory *TransactionFactoryPosgresql_deprec) adaptMarbleDatabaseTransaction(ctx context.Context, transaction Transaction_deprec) TransactionPostgres_deprec {

	if transaction == nil {
		transaction = TransactionPostgres_deprec{
			databaseShema: models.DATABASE_MARBLE_SCHEMA,
			exec:          factory.marbleConnectionPool,
		}
	}

	tx := transaction.(TransactionPostgres_deprec)

	if transaction.DatabaseSchema().SchemaType != models.DATABASE_SCHEMA_TYPE_MARBLE {
		panic("can only handle transactions in Marble database.")
	}
	return tx
}

func adaptClientDatabaseTransaction(transaction Transaction_deprec) TransactionPostgres_deprec {

	tx := transaction.(TransactionPostgres_deprec)
	if transaction.DatabaseSchema().SchemaType != models.DATABASE_SCHEMA_TYPE_CLIENT {
		panic("can only handle transactions in Client database.")
	}
	return tx
}

func (factory *TransactionFactoryPosgresql_deprec) Transaction(ctx context.Context, databaseSchema models.DatabaseSchema, fn func(tx Transaction_deprec) error) error {
	connPool, err := factory.databaseConnectionPoolRepository.DatabaseConnectionPool(ctx, databaseSchema.Database.Connection)
	if err != nil {
		return err
	}

	err = pgx.BeginFunc(ctx, connPool, func(tx pgx.Tx) error {
		return fn(TransactionPostgres_deprec{
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
