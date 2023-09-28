package repositories

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/checkmarble/marble-backend/models"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type IngestionRepository interface {
	IngestObjects(transaction Transaction, payloads []models.PayloadReader, table models.Table, logger *slog.Logger) (err error)
}

type IngestionRepositoryImpl struct {
}

func (repo *IngestionRepositoryImpl) IngestObjects(transaction Transaction, payloads []models.PayloadReader, table models.Table, logger *slog.Logger) (err error) {

	tx := adaptClientDatabaseTransaction(transaction)

	for _, payload := range payloads {
		objectIdItf, _ := payload.ReadFieldFromPayload("object_id")
		updatedAtItf, _ := payload.ReadFieldFromPayload("updated_at")
		objectId := objectIdItf.(string)
		updatedAt := updatedAtItf.(time.Time)

		previousIsMoreRecent, err := updateExistingVersionIfPresent(tx, objectId, updatedAt, table)
		if err != nil {
			return fmt.Errorf("error updating existing version: %w", err)
		} else if previousIsMoreRecent {
			logger.Debug(fmt.Sprintf("Previous version was more recent than %v, skipping", updatedAt), slog.String("type", tableNameWithSchema(tx, table.Name)), slog.String("id", objectId))
			continue
		}

		columnNames, values := generateInsertValues(table, payload)
		columnNames = append(columnNames, "id")
		values = append(values, uuid.NewString())

		sql := NewQueryBuilder().Insert(tableNameWithSchema(tx, table.Name)).Columns(columnNames...).Values(values...)

		_, err = tx.ExecBuilder(sql)
		if err != nil {
			return err
		}
		logger.Debug("Created object in db", slog.String("type", tableNameWithSchema(tx, table.Name)), slog.String("id", objectId))
	}
	return nil
}

func generateInsertValues(table models.Table, payload models.PayloadReader) (columnNames []string, values []interface{}) {
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

func updateExistingVersionIfPresent(tx TransactionPostgres, objectId string, updatedAt time.Time, table models.Table) (bool, error) {
	sql, args, err := NewQueryBuilder().
		Select("id, updated_at").
		From(tableNameWithSchema(tx, table.Name)).
		Where(squirrel.Eq{"object_id": objectId}).
		Where(squirrel.Eq{"valid_until": "Infinity"}).
		ToSql()
	if err != nil {
		return false, err
	}

	var id string
	var prevUpdatedAt time.Time
	err = tx.exec.QueryRow(tx.ctx, sql, args...).Scan(&id, &prevUpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	// "time.Before" is a strict inequality: if the timestamps are the same, proceed to update
	if updatedAt.Before(prevUpdatedAt) {
		return true, nil
	}

	sql, args, err = NewQueryBuilder().
		Update(tableNameWithSchema(tx, table.Name)).
		Set("valid_until", "now()").
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return false, err
	}
	_, err = tx.SqlExec(sql, args...)
	if err != nil {
		return false, err
	}

	return false, nil
}
