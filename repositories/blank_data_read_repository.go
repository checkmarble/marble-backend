package repositories

import (
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type BlankDataReadRepository interface {
	GetFirstTransactionTimestamp(transaction Transaction, accountId string) (time.Time, error)
	SumTransactionsAmount(transaction Transaction, accountId string, direction string, createdFrom time.Time, createdTo time.Time) (float64, error)
}

type BlankDataReadRepositoryImpl struct{}

func (repo *BlankDataReadRepositoryImpl) GetFirstTransactionTimestamp(transaction Transaction, accountId string) (time.Time, error) {
	tx := adaptClientDatabaseTransaction(transaction)

	tableName := tableNameWithSchema(tx, models.TableName("transactions"))
	query := NewQueryBuilder().
		Select("MIN(created_at) AS first_transaction_at").
		From(tableName).
		Where(squirrel.Eq{"account_id": accountId}).
		Where(rowIsValid(tableName))

	sql, args, err := query.ToSql()
	if err != nil {
		return time.Time{}, err
	}
	row := tx.exec.QueryRow(tx.ctx, sql, args...)

	var output time.Time
	err = row.Scan(&output)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, fmt.Errorf("no rows scanned while reading DB: %w", models.NotFoundError)
	} else if err != nil {
		return time.Time{}, err
	}
	return output, nil
}

func (repo *BlankDataReadRepositoryImpl) SumTransactionsAmount(transaction Transaction, accountId string, direction string, createdFrom time.Time, createdTo time.Time) (float64, error) {
	tx := adaptClientDatabaseTransaction(transaction)

	tableName := tableNameWithSchema(tx, models.TableName("transactions"))
	query := NewQueryBuilder().
		Select("SUM(txn_amount)").
		From(tableName).
		Where(squirrel.Eq{"account_id": accountId}).
		Where(squirrel.Eq{"direction": direction}).
		Where(squirrel.GtOrEq{"created_at": createdFrom}).
		Where(squirrel.LtOrEq{"created_at": createdTo}).
		Where(rowIsValid(tableName))

	sql, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}
	row := tx.exec.QueryRow(tx.ctx, sql, args...)

	var output float64
	err = row.Scan(&output)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("no rows scanned while reading DB: %w", models.NotFoundError)
	} else if err != nil {
		return 0, err
	}
	return output, nil
}
