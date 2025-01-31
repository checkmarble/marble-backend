package executor_factory

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/pashagolub/pgxmock/v4"
)

type ExecutorFactoryStub struct {
	Mock pgxmock.PgxPoolIface
}

func NewExecutorFactoryStub() ExecutorFactoryStub {
	pool, _ := pgxmock.NewPool()

	return ExecutorFactoryStub{
		Mock: pool,
	}
}

type PgExecutorStub struct {
	pgxmock.PgxPoolIface
}

func (stub ExecutorFactoryStub) NewClientDbExecutor(ctx context.Context, organizationId string) (repositories.Executor, error) {
	return nil, nil
}

func (stub ExecutorFactoryStub) NewExecutor() repositories.Executor {
	return PgExecutorStub{
		stub.Mock,
	}
}

func (stub PgExecutorStub) DatabaseSchema() models.DatabaseSchema {
	return models.DatabaseSchema{}
}
