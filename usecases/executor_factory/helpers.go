package executor_factory

import (
	"context"

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
