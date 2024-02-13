package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type BlankDataReadRepository interface {
	GetFirstTransactionTimestamp(ctx context.Context, transaction Transaction_deprec, ownerBusinessId string) (*time.Time, error)
	SumTransactionsAmount(ctx context.Context, transaction Transaction_deprec, ownerBusinessId string, direction string, createdFrom time.Time, createdTo time.Time) (float64, error)
	RetrieveTransactions(ctx context.Context, transaction Transaction_deprec, filters map[string]any, createdFrom time.Time) ([]map[string]any, error)
}

type BlankDataReadRepositoryImpl struct{}

func (repo *BlankDataReadRepositoryImpl) GetFirstTransactionTimestamp(ctx context.Context, transaction Transaction_deprec, ownerBusinessId string) (*time.Time, error) {
	tx := adaptClientDatabaseTransaction(transaction)

	tableName := tableNameWithSchema(tx, models.TableName("transactions"))
	query := NewQueryBuilder().
		Select("MIN(created_at) AS first_transaction_at").
		From(tableName).
		Where(squirrel.Eq{"owner_business_id": ownerBusinessId}).
		Where(rowIsValid(tableName))

	sql, args, err := query.ToSql()
	if err != nil {
		return &time.Time{}, err
	}
	row := tx.exec.QueryRow(ctx, sql, args...)

	var output *time.Time
	err = row.Scan(&output)
	if errors.Is(err, pgx.ErrNoRows) {
		return &time.Time{}, fmt.Errorf("no rows scanned while reading DB: %w", models.NotFoundError)
	} else if err != nil {
		return &time.Time{}, err
	}
	return output, nil
}

func (repo *BlankDataReadRepositoryImpl) SumTransactionsAmount(ctx context.Context, transaction Transaction_deprec, ownerBusinessId string, direction string, createdFrom time.Time, createdTo time.Time) (float64, error) {
	tx := adaptClientDatabaseTransaction(transaction)

	tableName := tableNameWithSchema(tx, models.TableName("transactions"))
	query := NewQueryBuilder().
		Select("COALESCE(SUM(txn_amount), 0)").
		From(tableName).
		Where(squirrel.Eq{"owner_business_id": ownerBusinessId}).
		Where(squirrel.Eq{"direction": direction}).
		Where(squirrel.GtOrEq{"created_at": createdFrom}).
		Where(squirrel.LtOrEq{"created_at": createdTo}).
		Where(rowIsValid(tableName))

	sql, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}
	row := tx.exec.QueryRow(ctx, sql, args...)

	var output float64
	err = row.Scan(&output)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("no rows scanned while reading DB: %w", models.NotFoundError)
	} else if err != nil {
		return 0, err
	}
	return output, nil
}

func (repo *BlankDataReadRepositoryImpl) RetrieveTransactions(ctx context.Context, transaction Transaction_deprec, filters map[string]any, createdFrom time.Time) ([]map[string]any, error) {
	tx := adaptClientDatabaseTransaction(transaction)

	tableName := tableNameWithSchema(tx, models.TableName("transactions"))
	query := NewQueryBuilder().
		Select("txn_amount, created_at, counterparty_iban").
		From(tableName).
		Where(squirrel.GtOrEq{"created_at": createdFrom}).
		Where(rowIsValid(tableName)).
		OrderBy("created_at DESC")
	for k, v := range filters {
		query = query.Where(squirrel.Eq{k: v})
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := tx.exec.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var output []map[string]any

	for rows.Next() {
		var txnAmount, createdAt, counterpartyIban any
		err := rows.Scan(&txnAmount, &createdAt, &counterpartyIban)
		if err != nil {
			return nil, err
		}
		output = append(output, map[string]any{
			"txn_amount":        txnAmount,
			"created_at":        createdAt,
			"counterparty_iban": counterpartyIban,
		})
	}
	return output, rows.Err()
}
