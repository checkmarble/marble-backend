package repositories

import (
	"context"
	"errors"
	"marble/marble-backend/models"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Opaque type, down-casted to TransactionPostgres in by repositories
// We may get ride of it to replace it by TransactionPostgres.
type Transaction interface {
	Database() models.Database
}

type TransactionPostgres struct {
	Target models.Database
	ctx    context.Context
	exec   TransactionOrPool // Optional, used when no transaction is provided
}

type TransactionOrPool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (tx TransactionPostgres) Database() models.Database {
	return tx.Target
}

// helper
func (transaction *TransactionPostgres) Exec(sql string, arguments ...any) (pgconn.CommandTag, error) {
	return transaction.exec.Exec(transaction.ctx, sql, arguments...)
}

// helper
func (transaction *TransactionPostgres) Query(sql string, arguments ...any) (pgx.Rows, error) {
	return transaction.exec.Query(transaction.ctx, sql, arguments...)
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

var ErrIgnoreRoolBackError = errors.New("ignore rollback error")

func IsIsUniqueViolationError(err error) bool {
	var pgxErr *pgconn.PgError
	return errors.As(err, &pgxErr) && pgxErr.Code == pgerrcode.UniqueViolation
}
