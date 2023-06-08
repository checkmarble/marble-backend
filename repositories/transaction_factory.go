package repositories

import (
	"context"
	"errors"
	"marble/marble-backend/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionFactory interface {
	Transaction(databaseSchema models.DatabaseSchema, fn func(tx Transaction) error) error
	adaptMarbleDatabaseTransaction(transaction Transaction) TransactionPostgres
}

type TransactionFactoryPosgresql struct {
	databaseConnectionPoolRepository DatabaseConnectionPoolRepository
	marbleConnectionPool             *pgxpool.Pool
}

func (factory *TransactionFactoryPosgresql) adaptMarbleDatabaseTransaction(transaction Transaction) TransactionPostgres {

	if transaction == nil {
		transaction = TransactionPostgres{
			databaseShema: models.DATABASE_MARBLE_SCHEMA,
			ctx:           context.Background(),
			exec:          factory.marbleConnectionPool,
		}
	}

	return adaptMarbleDatabaseTransaction(transaction)
}

func adaptMarbleDatabaseTransaction(transaction Transaction) TransactionPostgres {

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

func (repo *TransactionFactoryPosgresql) Transaction(databaseSchema models.DatabaseSchema, fn func(tx Transaction) error) error {
	connPool, err := repo.databaseConnectionPoolRepository.DatabaseConnectionPool(databaseSchema.Database.Connection)
	if err != nil {
		return err
	}

	// context.Background: I suppose we don't need cancellation at the sql request level.
	ctx := context.Background()

	err = pgx.BeginFunc(ctx, connPool, func(tx pgx.Tx) error {
		return fn(TransactionPostgres{
			databaseShema: databaseSchema,
			ctx:           ctx,
			exec:          tx,
		})
	})

	// helper: The callback can return ErrIgnoreRoolBackError
	// to explicitly specify that the error should be ignored.
	if errors.Is(err, ErrIgnoreRoolBackError) {
		return nil
	}

	return err
}
