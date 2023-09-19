package transaction

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

// Helper for TransactionFactory.Transaction that return something and an error:
// TransactionReturnValue and the callback fn returns (Model, error)
// Example:
// return transaction.TransactionReturnValue(
//
//	 usecase.transactionFactory,
//	 models.DATABASE_MARBLE_SCHEMA,
//	 func(tx repositories.Transaction) ([]models.User, error) {
//		return usecase.userRepository.Users(tx)
//	 },
//
// )
func TransactionReturnValue[ReturnType any](factory TransactionFactory, databaseSchema models.DatabaseSchema, fn func(tx repositories.Transaction) (ReturnType, error)) (ReturnType, error) {
	var value ReturnType
	transactionErr := factory.Transaction(databaseSchema, func(tx repositories.Transaction) error {
		var fnErr error
		value, fnErr = fn(tx)
		return fnErr
	})
	return value, transactionErr
}
