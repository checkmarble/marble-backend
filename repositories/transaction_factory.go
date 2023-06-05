package repositories

import (
	"context"
	"errors"
	"marble/marble-backend/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionFactory interface {
	Transaction(database models.Database, fn func(tx Transaction) error) error
	ImplicitTransactionOnMarbleDatabase() Transaction
}

type TransactionFactoryPosgresql struct {
	databaseConnectionPoolRepository DatabaseConnectionPoolRepository
	marbleConnectionPool             *pgxpool.Pool
}

func (repo *TransactionFactoryPosgresql) Transaction(database models.Database, fn func(tx Transaction) error) error {
	connPool, err := repo.databaseConnectionPoolRepository.DatabaseConnectionPool(database)
	if err != nil {
		return err
	}

	// context.Background: I suppose we don't need cancellation at the sql request level.
	ctx := context.Background()

	err = pgx.BeginFunc(ctx, connPool, func(tx pgx.Tx) error {
		return fn(TransactionPostgres{
			Target: database,
			ctx:    ctx,
			exec:   tx,
		})
	})

	// helper: The callback can return ErrIgnoreRoolBackError
	// to explicitly specify that the error should be ignored.
	if errors.Is(err, ErrIgnoreRoolBackError) {
		return nil
	}

	return err
}

func (repo *TransactionFactoryPosgresql) ImplicitTransactionOnMarbleDatabase() Transaction {
	return TransactionPostgres{
		Target: models.DATABASE_MARBLE,
		ctx:    context.Background(),
		exec:   repo.marbleConnectionPool,
	}
}
