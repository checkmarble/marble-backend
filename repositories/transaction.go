package repositories

import (
	"context"
	"marble/marble-backend/models"

	"github.com/jackc/pgx/v5"
)

// Opaque type, down-casted to TransactionPostgres in by repositories
// We may get ride of it to replace it by TransactionPostgres.
type Transaction interface {
	Database() models.Database
}

type TransactionPostgres struct {
	Target models.Database
	ctx    context.Context
	tx     pgx.Tx
}

func (tx TransactionPostgres) Database() models.Database {
	return tx.Target
}

// Helper for TransactionFactory.Transaction that return something and an error:
// TransactionReturnValue and the callback fn returns (Model, error)
// Example:
// return repositories.TransactionReturnValue(
//
//	 usecase.transactionFactory,
//	 models.DATABASE_MARBLE,
//	 func(tx repositories.Transaction) ([]models.User, error) {
//		return usecase.userRepository.Users(tx)
//	 },
//
// )
func TransactionReturnValue[ReturnType any](factory TransactionFactory, database models.Database, fn func(tx Transaction) (ReturnType, error)) (ReturnType, error) {
	var value ReturnType
	transactionErr := factory.Transaction(database, func(tx Transaction) error {
		var fnErr error
		value, fnErr = fn(tx)
		return fnErr
	})
	return value, transactionErr
}
