package repositories

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
)

type IngestionRepository interface {
	IngestObjects(transaction Transaction, payloads []models.PayloadReader, table models.Table, logger *slog.Logger) (err error)
}

type IngestionRepositoryImpl struct {
}

func (repo *IngestionRepositoryImpl) IngestObjects(transaction Transaction, payloads []models.PayloadReader, table models.Table, logger *slog.Logger) (err error) {
	tx := adaptClientDatabaseTransaction(transaction)

	mostRecentObjectIds, mostRecentPayloads := repo.mostRecentPayloadsByObjectId(payloads)

	previouslyIngestedObjects, err := repo.loadPreviouslyIngestedObjects(tx, mostRecentObjectIds, table.Name)
	if err != nil {
		return err
	}

	payloadsToInsert, obsoleteIngestedObjectIds := repo.comparePayloadsToIngestedObjects(mostRecentPayloads, previouslyIngestedObjects)

	if len(obsoleteIngestedObjectIds) > 0 {
		if err := repo.batchUpdateValidUntilOnObsoleteObjects(tx, table.Name, obsoleteIngestedObjectIds); err != nil {
			return err
		}
	}

	if len(payloadsToInsert) > 0 {
		if err := repo.batchInsertPayloadsAndEnumValues(tx, payloadsToInsert, table); err != nil {
			return err
		}
	}

	logger.Info("Inserted objects in db", slog.String("type", tableNameWithSchema(tx, table.Name)), slog.Int("nb_objects", len(payloadsToInsert)))

	return nil
}

func objectIdAndUpdatedAtFromPayload(payload models.PayloadReader) (string, time.Time) {
	objectIdItf, _ := payload.ReadFieldFromPayload("object_id")
	updatedAtItf, _ := payload.ReadFieldFromPayload("updated_at")
	objectId := objectIdItf.(string)
	updatedAt := updatedAtItf.(time.Time)

	return objectId, updatedAt
}

func (repo *IngestionRepositoryImpl) mostRecentPayloadsByObjectId(payloads []models.PayloadReader) ([]string, []models.PayloadReader) {
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

	mostRecentPayloads := make([]models.PayloadReader, 0, len(recentMap))
	mostRecentObjectIds := make([]string, 0, len(recentMap))
	for key, obj := range recentMap {
		mostRecentObjectIds = append(mostRecentObjectIds, key)
		mostRecentPayloads = append(mostRecentPayloads, obj)
	}

	return mostRecentObjectIds, mostRecentPayloads
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

	return SqlToListOfModels(tx, query, func(db DBObject) (IngestedObject, error) { return IngestedObject(db), nil })
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

func (repo *IngestionRepositoryImpl) batchInsertPayloadsAndEnumValues(tx TransactionPostgres, payloads []models.PayloadReader, table models.Table) error {
	columnNames := models.ColumnNames(table)
	query := NewQueryBuilder().Insert(tableNameWithSchema(tx, table.Name))

	enumValues := repo.buildEnumValuesWithEnumFields(table)

	for _, payload := range payloads {
		repo.collectEnumValues(payload, enumValues)

		insertValues, err := repo.generateInsertValues(payload, columnNames)
		if err != nil {
			return fmt.Errorf("generateInsertValues error: %w", err)
		}
		insertValues = append(insertValues, uuid.NewString())
		query = query.Values(insertValues...)
	}

	err := repo.batchInsertEnumValues(tx, enumValues, table)
	if err != nil {
		return fmt.Errorf("batchInsertEnumValues error: %w", err)
	}

	columnNames = append(columnNames, "id")
	query = query.Columns(columnNames...)

	_, err = tx.ExecBuilder(query)

	return err
}

type EnumValues map[string]map[any]bool

func (repo *IngestionRepositoryImpl) buildEnumValuesWithEnumFields(table models.Table) EnumValues {
	enumValues := make(EnumValues)
	for fieldName := range table.Fields {
		dataType := table.Fields[fieldName].DataType
		if table.Fields[fieldName].IsEnum && (dataType == models.String || dataType == models.Float) {
			enumValues[string(fieldName)] = make(map[any]bool)
		}
	}
	return enumValues
}

func (repo *IngestionRepositoryImpl) generateInsertValues(payload models.PayloadReader, columnNames []string) ([]interface{}, error) {
	insertValues := make([]interface{}, len(columnNames))
	i := 0
	for _, columnName := range columnNames {
		fieldName := models.FieldName(columnName)
		insertValues[i], _ = payload.ReadFieldFromPayload(fieldName)
		i++
	}
	return insertValues, nil
}

func (repo *IngestionRepositoryImpl) collectEnumValues(payload models.PayloadReader, enumValues EnumValues) {
	for fieldName := range enumValues {
		value, _ := payload.ReadFieldFromPayload(models.FieldName(fieldName))

		if value != nil && value != "" {
			enumValues[fieldName][value] = true
		}
	}
}

// This has to be done in 2 queries because there cannot be multiple ON CONFLICT clauses per query
func (repo *IngestionRepositoryImpl) batchInsertEnumValues(tx TransactionPostgres, enumValues EnumValues, table models.Table) error {
	textQuery := NewQueryBuilder().
		Insert("data_model_enum_values").
		Columns("field_id", "text_value").
		Suffix("ON CONFLICT ON CONSTRAINT unique_data_model_enum_text_values_field_id_value DO UPDATE SET last_seen = NOW()")

	floatQuery := NewQueryBuilder().
		Insert("data_model_enum_values").
		Columns("field_id", "float_value").
		Suffix("ON CONFLICT ON CONSTRAINT unique_data_model_enum_float_values_field_id_value DO UPDATE SET last_seen = NOW()")

	// Hack to avoid empty query, which would cause an execution error
	var shouldInsertTextValues bool
	var shouldInsertFloatValues bool

	for fieldName, values := range enumValues {
		fieldId := table.Fields[models.FieldName(fieldName)].ID
		dataType := table.Fields[models.FieldName(fieldName)].DataType

		for value := range values {
			if dataType == models.String {
				textQuery = textQuery.Values(fieldId, value)
				shouldInsertTextValues = true
			} else if dataType == models.Float {
				floatQuery = floatQuery.Values(fieldId, value)
				shouldInsertFloatValues = true
			}
		}
	}

	if shouldInsertTextValues {
		_, err := tx.ExecBuilder(textQuery)
		if err != nil {
			return err
		}
	}
	if shouldInsertFloatValues {
		_, err := tx.ExecBuilder(floatQuery)
		if err != nil {
			return err
		}
	}

	return nil
}
