package repositories

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
)

type IngestionRepository interface {
	IngestObjects(transaction Transaction, payloads []models.PayloadReader, table models.Table, logger *slog.Logger) (err error)
}

type IngestionRepositoryImpl struct {
}

func (repo *IngestionRepositoryImpl) IngestObjects(transaction Transaction, payloads []models.PayloadReader, table models.Table, logger *slog.Logger) (err error) {
	tx := adaptClientDatabaseTransaction(transaction)

	// Iterate over payloads to keep only most recent records
	recentObjectIds, recentPayloads := repo.filterMostRecentPayloads(payloads)

	// Load all previously ingested objects
	previouslyIngestedObjects, err := repo.loadPreviouslyIngestedObjects(tx, recentObjectIds, table.Name)
	if err != nil {
		return err
	}

	// Iterate over payloads and compare with previously ingested objects
	payloadsToInsert, obsoleteIngestedObjectIds := repo.comparePayloadsToIngestedObjects(recentPayloads, previouslyIngestedObjects)

	// Batch update valid_until for obsolete objects
	if err := repo.batchUpdateValidUntilOnObsoleteObjects(tx, table.Name, obsoleteIngestedObjectIds); err != nil {
		return err
	}

	// Batch insert the new payloads
	if err := repo.batchInsertPayloads(tx, payloadsToInsert, table); err != nil {
		return err
	}

	logger.Debug("Inserted objects in db", slog.String("type", tableNameWithSchema(tx, table.Name)), slog.Int("nb_objects", len(payloadsToInsert)))

	return nil
}

func objectIdAndUpdatedAtFromPayload(payload models.PayloadReader) (string, time.Time) {
	objectIdItf, _ := payload.ReadFieldFromPayload("object_id")
	updatedAtItf, _ := payload.ReadFieldFromPayload("updated_at")
	objectId := objectIdItf.(string)
	updatedAt := updatedAtItf.(time.Time)

	return objectId, updatedAt
}

func (repo *IngestionRepositoryImpl) filterMostRecentPayloads(payloads []models.PayloadReader) ([]string, []models.PayloadReader) {
	recentMap := make(map[string]models.PayloadReader)
	for _, payload := range payloads {
		objectId, updatedAt := objectIdAndUpdatedAtFromPayload(payload)

		if seen, ok := recentMap[objectId]; ok {
			_, seenUpdatedAt := objectIdAndUpdatedAtFromPayload(seen)
			if updatedAt.After(seenUpdatedAt) {
				recentMap[objectId] = payload
			}
		} else {
			recentMap[objectId] = payload
		}
	}

	recentPayloads := make([]models.PayloadReader, 0, len(recentMap))
	recentObjectIds := make([]string, 0, len(recentMap))
	for key, obj := range recentMap {
		recentObjectIds = append(recentObjectIds, key)
		recentPayloads = append(recentPayloads, obj)
	}

	return recentObjectIds, recentPayloads
}

type DBObject struct {
	Id        string    `db:"id"`
	ObjectId  string    `db:"object_id"`
	UpdatedAt time.Time `db:"updated_at"`
}
type IngestedObject struct {
	Id        string
	ObjectId  string
	UpdatedAt time.Time
}

func (repo *IngestionRepositoryImpl) loadPreviouslyIngestedObjects(tx TransactionPostgres, objectIds []string, tableName models.TableName) ([]IngestedObject, error) {
	query := NewQueryBuilder().
		Select("id, object_id, updated_at").
		From(tableNameWithSchema(tx, tableName)).
		Where(squirrel.Eq{"object_id": objectIds}).
		Where(squirrel.Eq{"valid_until": "Infinity"})

	previouslyIngestedObjects, err := SqlToListOfModels(tx, query, func(db DBObject) (IngestedObject, error) { return IngestedObject(db), nil })
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	return previouslyIngestedObjects, nil
}

func (repo *IngestionRepositoryImpl) comparePayloadsToIngestedObjects(payloads []models.PayloadReader, previouslyIngestedObjects []IngestedObject) ([]models.PayloadReader, []string) {
	previouslyIngestedMap := make(map[string]IngestedObject)
	for _, obj := range previouslyIngestedObjects {
		previouslyIngestedMap[obj.ObjectId] = obj
	}

	payloadsToInsert := make([]models.PayloadReader, 0, len(payloads))
	obsoleteIngestedObjectIds := make([]string, 0, len(previouslyIngestedMap))

	for _, payload := range payloads {
		objectId, updatedAt := objectIdAndUpdatedAtFromPayload(payload)

		existingObject, exists := previouslyIngestedMap[objectId]
		if !exists {
			payloadsToInsert = append(payloadsToInsert, payload)
		} else if updatedAt.After(existingObject.UpdatedAt) {
			payloadsToInsert = append(payloadsToInsert, payload)
			obsoleteIngestedObjectIds = append(obsoleteIngestedObjectIds, existingObject.Id)
		}
	}

	return payloadsToInsert, obsoleteIngestedObjectIds
}

func (repo *IngestionRepositoryImpl) batchUpdateValidUntilOnObsoleteObjects(tx TransactionPostgres, tableName models.TableName, obsoleteIngestedObjectIds []string) error {
	sql, args, err := NewQueryBuilder().
		Update(tableNameWithSchema(tx, tableName)).
		Set("valid_until", "now()").
		Where(squirrel.Eq{"id": obsoleteIngestedObjectIds}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = tx.SqlExec(sql, args...)

	return err
}

// TODO refacto the insertion of enum values into one db commit
func generateInsertValues(tx TransactionPostgres, table models.Table, payload models.PayloadReader, columnNames []string) ([]interface{}, error) {
	insertValues := make([]interface{}, len(columnNames))
	i := 0
	for _, columnName := range columnNames {
		fieldName := models.FieldName(columnName)
		insertValues[i], _ = payload.ReadFieldFromPayload(fieldName)

		// Check for enum values
		dataType := table.Fields[fieldName].DataType
		if table.Fields[fieldName].IsEnum && insertValues[i] != nil && (dataType == models.String || dataType == models.Float) {
			err := addEnumValue(tx, table.Fields[fieldName].ID, dataType, insertValues[i])
			if err != nil {
				return nil, fmt.Errorf("addEnumValue error: %w", err)
			}
		}
		i++
	}
	return insertValues, nil
}

func addEnumValue(tx TransactionPostgres, fieldID string, dataType models.DataType, value interface{}) error {
	dataTypeEnumColumnMap := map[models.DataType]string{
		models.String: "text_value",
		models.Float:  "float_value",
	}
	dataTypeEnumColumnConstraintMap := map[models.DataType]string{
		models.String: "unique_data_model_enum_text_values_field_id_value",
		models.Float:  "unique_data_model_enum_float_values_field_id_value",
	}
	column, ok1 := dataTypeEnumColumnMap[dataType]
	constraint, ok2 := dataTypeEnumColumnConstraintMap[dataType]
	if !ok1 || !ok2 {
		return fmt.Errorf("addEnumValue error: data type %s can't handle enum values", dataType.String())
	}

	query := fmt.Sprintf(`
		INSERT INTO data_model_enum_values (field_id, %s)
		VALUES ($1, $2)
		ON CONFLICT ON CONSTRAINT %s
		DO UPDATE SET last_seen = NOW()
	`, column, constraint)

	_, err := tx.exec.Exec(tx.ctx, query, fieldID, value)
	if err != nil {
		return err
	}
	return nil
}

func (repo *IngestionRepositoryImpl) batchInsertPayloads(tx TransactionPostgres, payloads []models.PayloadReader, table models.Table) error {
	columnNames := models.ColumnNames(table)
	query := NewQueryBuilder().Insert(tableNameWithSchema(tx, table.Name))

	for _, payload := range payloads {
		insertValues, err := generateInsertValues(tx, table, payload, columnNames)
		if err != nil {
			return fmt.Errorf("generateInsertValues error: %w", err)
		}
		insertValues = append(insertValues, uuid.NewString())
		query = query.Values(insertValues...)
	}

	columnNames = append(columnNames, "id")
	query = query.Columns(columnNames...)

	_, err := tx.ExecBuilder(query)
	if err != nil {
		return err
	}

	return nil
}
