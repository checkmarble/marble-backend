package repositories

import (
	"errors"
	"fmt"
	"marble/marble-backend/models"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/exp/slog"
)

type IngestionRepository interface {
	IngestObject(transaction Transaction, payload models.Payload, table models.Table, logger *slog.Logger) (err error)
}

type IngestionRepositoryImpl struct {
}

func (repo *IngestionRepositoryImpl) IngestObject(transaction Transaction, payload models.Payload, table models.Table, logger *slog.Logger) (err error) {

	tx := adaptClientDatabaseTransaction(transaction)

	err = updateExistingVersionIfPresent(tx, payload, table)
	if err != nil {
		return fmt.Errorf("Error updating existing version: %w", err)
	}

	columnNames, values := generateInsertValues(table, payload)
	columnNames = append(columnNames, "id")
	values = append(values, uuid.NewString())

	sql := NewQueryBuilder().Insert(tableNameWithSchema(tx, table.Name)).Columns(columnNames...).Values(values...).Suffix("RETURNING \"id\"")

	var createdObjectID string
	_, err = tx.ExecBuilder(sql)
	if err != nil {
		return err
	}
	logger.Info("Created object in db", slog.String("type", tableNameWithSchema(tx, table.Name)), slog.String("object_id", createdObjectID))

	return nil
}

func generateInsertValues(table models.Table, payload models.Payload) (columnNames []string, values []interface{}) {
	nbFields := len(table.Fields)
	columnNames = make([]string, nbFields)
	values = make([]interface{}, nbFields)
	i := 0
	for fieldName := range table.Fields {
		columnNames[i] = string(fieldName)
		values[i], _ = payload.ReadFieldFromPayload(fieldName)
		i++
	}
	return columnNames, values
}

func updateExistingVersionIfPresent(
	tx TransactionPostgres,
	payload models.Payload,
	table models.Table) (err error) {

	object_id, _ := payload.ReadFieldFromPayload("object_id")
	sql, args, err := NewQueryBuilder().
		Select("id").
		From(tableNameWithSchema(tx, table.Name)).
		Where(squirrel.Eq{"object_id": object_id}).
		Where(squirrel.Eq{"valid_until": "Infinity"}).
		ToSql()
	if err != nil {
		return err
	}

	var id string
	err = tx.exec.QueryRow(tx.ctx, sql, args...).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}

	sql, args, err = NewQueryBuilder().
		Update(tableNameWithSchema(tx, table.Name)).
		Set("valid_until", "now()").
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = tx.SqlExec(sql, args...)
	if err != nil {
		return err
	}

	return nil
}
