package db_executor_factory

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

// helper with generics
func TransactionReturnValueInOrgSchema[ReturnType any](
	ctx context.Context,
	factory DbExecutorFactory,
	organizationId string,
	fn func(tx repositories.Transaction) (ReturnType, error),
) (ReturnType, error) {
	var value ReturnType
	transactionErr := factory.TransactionInOrgSchema(ctx, organizationId, func(tx *repositories.ExecutorPostgres) error {
		var fnErr error
		value, fnErr = fn(tx)
		return fnErr
	})
	return value, transactionErr
}

// helper with generics
func TransactionReturnValue[ReturnType any](
	ctx context.Context,
	factory DbExecutorFactory,
	databaseSchema models.DatabaseSchema,
	fn func(tx repositories.Transaction) (ReturnType, error),
) (ReturnType, error) {
	var value ReturnType
	transactionErr := factory.Transaction(ctx, func(tx *repositories.ExecutorPostgres) error {
		var fnErr error
		value, fnErr = fn(tx)
		return fnErr
	})
	return value, transactionErr
}
