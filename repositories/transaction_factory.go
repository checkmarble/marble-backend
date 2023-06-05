package repositories

import (
	"context"
	"errors"
	"marble/marble-backend/models"

	"github.com/jackc/pgx/v5"
)

type TransactionFactory interface {
	Transaction(database models.Database, fn func(tx Transaction) error) error
}

type TransactionFactoryPosgresql struct {
	DatabaseConnectionPoolRepository DatabaseConnectionPoolRepository
}

func (t *TransactionFactoryPosgresql) Transaction(database models.Database, fn func(tx Transaction) error) error {
	connPool, err := t.DatabaseConnectionPoolRepository.DatabaseConnectionPool(database)
	if err != nil {
		return err
	}

	// context.Background: I suppose we don't need cancellation at the sql request level.
	ctx := context.Background()

	err = pgx.BeginFunc(ctx, connPool, func(tx pgx.Tx) error {
		return fn(TransactionPostgres{
			Target: database,
			ctx:    ctx,
			tx:     tx,
		})
	})

	// helper: The callback can return ErrIgnoreRoolBackError
	// to explicitly specify that the error should be ignored.
	if errors.Is(err, ErrIgnoreRoolBackError) {
		return nil
	}

	return err
}
