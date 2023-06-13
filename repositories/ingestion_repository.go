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
	queryBuilder squirrel.StatementBuilderType
}

func (repo *IngestionRepositoryImpl) IngestObject(transaction Transaction, payloadStructWithReader models.Payload, table models.Table, logger *slog.Logger) (err error) {

	tx := adaptClientDatabaseTransaction(transaction)

	err = updateExistingVersionIfPresent(tx, repo.queryBuilder, payloadStructWithReader, table)
	if err != nil {
		return fmt.Errorf("Error updating existing version: %w", err)
	}

	columnNames, values := generateInsertValues(table, payloadStructWithReader)
	columnNames = append(columnNames, "id")
	values = append(values, uuid.NewString())

	sql := repo.queryBuilder.Insert(tableNameWithSchema(tx, table.Name)).Columns(columnNames...).Values(values...).Suffix("RETURNING \"id\"")

	var createdObjectID string
	_, err = tx.ExecBuilder(sql)
	if err != nil {
		return err
	}
	logger.Info("Created object in db", slog.String("type", tableNameWithSchema(tx, table.Name)), slog.String("object_id", createdObjectID))

	return nil
}

func generateInsertValues(table models.Table, payloadStructWithReader models.Payload) (columnNames []string, values []interface{}) {
	nbFields := len(table.Fields)
	columnNames = make([]string, nbFields)
	values = make([]interface{}, nbFields)
	i := 0
	for fieldName := range table.Fields {
		columnNames[i] = string(fieldName)
		values[i], _ = payloadStructWithReader.ReadFieldFromPayload(fieldName)
		i++
	}
	return columnNames, values
}

func updateExistingVersionIfPresent(
	tx TransactionPostgres,
	queryBuilder squirrel.StatementBuilderType,
	payloadStructWithReader models.Payload,
	table models.Table) (err error) {

	object_id, _ := payloadStructWithReader.ReadFieldFromPayload("object_id")
	sql, args, err := queryBuilder.
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

	sql, args, err = queryBuilder.
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
