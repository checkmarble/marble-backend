package executor_factory

import (
	"context"

	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
)

// helper with generics in org db schema
func TransactionReturnValueInOrgSchema[ReturnType any](
	ctx context.Context,
	factory TransactionFactory,
	organizationId uuid.UUID,
	fn func(tx repositories.Transaction) (ReturnType, error),
) (ReturnType, error) {
	var value ReturnType
	transactionErr := factory.TransactionInOrgSchema(ctx, organizationId, func(tx repositories.Transaction) error {
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
	fn func(tx repositories.Transaction) (ReturnType, error),
) (ReturnType, error) {
	var value ReturnType
	transactionErr := factory.Transaction(ctx, func(tx repositories.Transaction) error {
		var fnErr error
		value, fnErr = fn(tx)
		return fnErr
	})
	return value, transactionErr
}

func QueryGroup[R any](
	ctx context.Context,
	factory ExecutorFactory,
	skipAudit bool,
	fn func(conn repositories.Executor) (R, error),
) (value R, err error) {
	conn, release, err := factory.NewPinnedExecutor(ctx, skipAudit)
	if err != nil {
		return value, err
	}
	defer release()

	value, err = fn(conn)

	return
}
