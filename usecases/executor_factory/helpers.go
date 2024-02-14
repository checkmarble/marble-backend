package executor_factory

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

// helper with generics in org db schema
func TransactionReturnValueInOrgSchema[ReturnType any](
	ctx context.Context,
	factory TransactionFactory,
	organizationId string,
	fn func(tx repositories.Executor) (ReturnType, error),
) (ReturnType, error) {
	var value ReturnType
	transactionErr := factory.TransactionInOrgSchema(ctx, organizationId, func(tx repositories.Executor) error {
		var fnErr error
		value, fnErr = fn(tx)
		return fnErr
	})
	return value, transactionErr
}

// helper with generics in marble db schema
func TransactionReturnValue[ReturnType any](
	ctx context.Context,
	factory TransactionFactory,
	databaseSchema models.DatabaseSchema,
	fn func(tx repositories.Executor) (ReturnType, error),
) (ReturnType, error) {
	var value ReturnType
	transactionErr := factory.Transaction(ctx, func(tx repositories.Executor) error {
		var fnErr error
		value, fnErr = fn(tx)
		return fnErr
	})
	return value, transactionErr
}

type TransactionFactory interface {
	TransactionInOrgSchema(ctx context.Context, organizationId string, f func(tx repositories.Executor) error) error
	Transaction(ctx context.Context, fn func(tx repositories.Executor) error) error
	// Transaction(ctx context.Context, fn func(tx repositories.Executor) error) error
}

// Interface to be used in usecases, implemented by the DbExecutorFactory class in the usecases/db_executor_factory package
// which itself has the ExecutorGetter repository class injected in it.
type ClientSchemaExecutorFactory interface {
	NewClientDbExecutor(ctx context.Context, organizationId string) (repositories.Executor, error)
}
