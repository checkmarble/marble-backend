package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"

	"github.com/cockroachdb/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DatabaseConnectionPoolRepository interface {
	DatabaseConnectionPool(ctx context.Context, connection models.PostgresConnection) (*pgxpool.Pool, error)
}

type TransactionFactoryPosgresql struct {
	databaseConnectionPoolRepository DatabaseConnectionPoolRepository
	marbleConnectionPool             *pgxpool.Pool
}

func NewTransactionFactoryPosgresql(
	databaseConnectionPoolRepository DatabaseConnectionPoolRepository,
	marbleConnectionPool *pgxpool.Pool,
) TransactionFactoryPosgresql {
	return TransactionFactoryPosgresql{
		databaseConnectionPoolRepository: databaseConnectionPoolRepository,
		marbleConnectionPool:             marbleConnectionPool,
	}
}

func (factory *TransactionFactoryPosgresql) adaptMarbleDatabaseTransaction(ctx context.Context, transaction Transaction) TransactionPostgres {

	if transaction == nil {
		transaction = TransactionPostgres{
			databaseShema: models.DATABASE_MARBLE_SCHEMA,
			exec:          factory.marbleConnectionPool,
		}
	}

	tx := transaction.(TransactionPostgres)

	if transaction.DatabaseSchema().SchemaType != models.DATABASE_SCHEMA_TYPE_MARBLE {
		panic("can only handle transactions in Marble database.")
	}
	return tx
}

func adaptClientDatabaseTransaction(transaction Transaction) TransactionPostgres {

	tx := transaction.(TransactionPostgres)
	if transaction.DatabaseSchema().SchemaType != models.DATABASE_SCHEMA_TYPE_CLIENT {
		panic("can only handle transactions in Client database.")
	}
	return tx
}

func (factory *TransactionFactoryPosgresql) Transaction(ctx context.Context, databaseSchema models.DatabaseSchema, fn func(tx Transaction) error) error {
	connPool, err := factory.databaseConnectionPoolRepository.DatabaseConnectionPool(ctx, databaseSchema.Database.Connection)
	if err != nil {
		return err
	}

	err = pgx.BeginFunc(ctx, connPool, func(tx pgx.Tx) error {
		return fn(TransactionPostgres{
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
