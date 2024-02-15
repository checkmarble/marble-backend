package executor_factory

import (
	"context"

	"github.com/checkmarble/marble-backend/repositories"
)

type TransactionFactory interface {
	TransactionInOrgSchema(ctx context.Context, organizationId string,
		f func(tx repositories.Executor) error) error
	Transaction(ctx context.Context, fn func(tx repositories.Executor) error) error
}

// Interface to be used in usecases, implemented by the DbExecutorFactory class in the usecases/db_executor_factory package
// which itself has the ExecutorGetter repository class injected in it.
type ExecutorFactory interface {
	NewClientDbExecutor(ctx context.Context, organizationId string) (repositories.Executor, error)
	NewExecutor() repositories.Executor
}
