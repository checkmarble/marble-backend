package repositories

import (
	"context"
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

	return pgx.BeginFunc(ctx, connPool, func(tx pgx.Tx) error {
		return fn(TransactionPostgres{
			Target: database,
			ctx:    ctx,
			tx:     tx,
		})
	})
}
