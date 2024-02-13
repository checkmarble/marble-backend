package transaction

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

// Helper for TransactionFactory.Transaction that return something and an error:
// TransactionReturnValue_deprec and the callback fn returns (Model, error)
// Example:
// return transaction.TransactionReturnValue_deprec(
//
//	 usecase.transactionFactory,
//	 models.DATABASE_MARBLE_SCHEMA,
//	 func(tx repositories.Transaction) ([]models.User, error) {
//		return usecase.userRepository.Users(tx)
//	 },
//
// )
func TransactionReturnValue_deprec[ReturnType any](ctx context.Context, factory TransactionFactory_deprec, databaseSchema models.DatabaseSchema, fn func(tx repositories.Transaction_deprec) (ReturnType, error)) (ReturnType, error) {
	var value ReturnType
	transactionErr := factory.Transaction(ctx, databaseSchema, func(tx repositories.Transaction_deprec) error {
		var fnErr error
		value, fnErr = fn(tx)
		return fnErr
	})
	return value, transactionErr
}
