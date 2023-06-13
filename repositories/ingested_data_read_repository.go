package repositories

import (
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/operators"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type IngestedDataReadRepository interface {
	GetDbField(transaction Transaction, readParams models.DbFieldReadParams) (interface{}, error)
}

type IngestedDataReadRepositoryImpl struct {
	queryBuilder squirrel.StatementBuilderType
}

func (repo *IngestedDataReadRepositoryImpl) GetDbField(transaction Transaction, readParams models.DbFieldReadParams) (interface{}, error) {
	tx := adaptClientDatabaseTransaction(transaction)

	if len(readParams.Path) == 0 {
		return nil, fmt.Errorf("Path is empty: %w", operators.ErrDbReadInconsistentWithDataModel)
	}
	row, err := repo.queryDbForField(tx, readParams)
	if err != nil {
		return nil, fmt.Errorf("Error while building query for DB field: %w", err)
	}

	lastTable, err := getLastTableFromPath(readParams)
	if err != nil {
		return nil, err
	}
	fieldFromModel, ok := lastTable.Fields[models.FieldName(readParams.FieldName)]
	if !ok {
		return nil, fmt.Errorf("Field %s not found in table %s", readParams.FieldName, lastTable.Name)
	}

	switch fieldFromModel.DataType {
	case models.Bool:
		return scanRowReturnValue[pgtype.Bool](row)
	case models.Int:
		return scanRowReturnValue[pgtype.Int2](row)
	case models.Float:
		return scanRowReturnValue[pgtype.Float8](row)
	case models.String:
		return scanRowReturnValue[pgtype.Text](row)
	case models.Timestamp:
		return scanRowReturnValue[pgtype.Timestamp](row)
	default:
		return nil, fmt.Errorf("Unknown data type when reading from db: %s", fieldFromModel.DataType)
	}
}

func scanRowReturnValue[T pgtype.Bool | pgtype.Int2 | pgtype.Float8 | pgtype.Text | pgtype.Timestamp](row pgx.Row) (T, error) {
	var returnVariable T
	err := row.Scan(&returnVariable)
	if errors.Is(err, pgx.ErrNoRows) {
		return returnVariable, fmt.Errorf("No rows scanned while reading DB: %w", operators.OperatorNoRowsReadInDbError)
	} else if err != nil {
		return returnVariable, err
	}
	return returnVariable, nil
}

func (repo *IngestedDataReadRepositoryImpl) queryDbForField(tx TransactionPostgres, readParams models.DbFieldReadParams) (pgx.Row, error) {
	baseObjectId, err := getBaseObjectIdFromPayload(readParams.Payload)
	if err != nil {
		return nil, err
	}

	firstTable, ok := readParams.DataModel.Tables[readParams.TriggerTableName]
	if !ok {
		return nil, fmt.Errorf("Table %s not found in data model", readParams.TriggerTableName)
	}
	lastTable, err := getLastTableFromPath(readParams)
	if err != nil {
		return nil, err
	}

	firstTableName := tableNameWithSchema(tx, firstTable.Name)
	lastTableName := tableNameWithSchema(tx, lastTable.Name)

	// setup the end table we read the field from, the beginning table we join from, and relevant filters on the latter
	query := repo.queryBuilder.
		Select(fmt.Sprintf("%s.%s", lastTableName, readParams.FieldName)).
		From(firstTableName).
		Where(squirrel.Eq{fmt.Sprintf("%s.object_id", firstTableName): baseObjectId}).
		Where(rowIsValid(firstTableName))

	query, err = addJoinsOnIntermediateTables(tx, query, readParams, firstTable)
	if err != nil {
		return nil, err
	}

	sql, args, err := query.ToSql()

	if err != nil {
		return nil, fmt.Errorf("Error while building SQL query: %w", err)
	}

	row := tx.QueryRow(sql, args...)
	return row, nil
}

func getBaseObjectIdFromPayload(payload models.Payload) (string, error) {
	baseObjectIdAny, _ := payload.ReadFieldFromPayload("object_id")
	baseObjectIdPtr, ok := baseObjectIdAny.(*string)
	if !ok {
		return "", fmt.Errorf("object_id in payload is not a string") // should not happen, as per input validation
	}

	if baseObjectIdPtr == nil {
		return "", fmt.Errorf("object_id in payload is null") // should not happen, as per input validation
	}
	baseObjectId := *baseObjectIdPtr
	return baseObjectId, nil
}

func addJoinsOnIntermediateTables(tx TransactionPostgres, query squirrel.SelectBuilder, readParams models.DbFieldReadParams, firstTable models.Table) (squirrel.SelectBuilder, error) {
	currentTable := firstTable
	for _, linkName := range readParams.Path {
		link, ok := currentTable.LinksToSingle[linkName]
		if !ok {
			return squirrel.SelectBuilder{}, fmt.Errorf("No link with name %s on table %s: %w", linkName, currentTable.Name, operators.ErrDbReadInconsistentWithDataModel)
		}
		nextTable, ok := readParams.DataModel.Tables[link.LinkedTableName]
		if !ok {
			return squirrel.SelectBuilder{}, fmt.Errorf("No table with name %s: %w", link.LinkedTableName, operators.ErrDbReadInconsistentWithDataModel)
		}

		currentTableName := tableNameWithSchema(tx, currentTable.Name)
		nextTableName := tableNameWithSchema(tx, nextTable.Name)
		joinClause := fmt.Sprintf(
			"%s ON %s.%s = %s.%s",
			nextTableName,
			currentTableName,
			link.ChildFieldName,
			nextTableName,
			link.ParentFieldName)
		query = query.Join(joinClause).
			Where(rowIsValid(nextTableName))

		currentTable = nextTable
	}
	return query, nil
}

func rowIsValid(tableName string) squirrel.Eq {
	return squirrel.Eq{fmt.Sprintf("%s.valid_until", tableName): "Infinity"}
}

func getLastTableFromPath(params models.DbFieldReadParams) (models.Table, error) {
	firstTable, ok := params.DataModel.Tables[params.TriggerTableName]
	if !ok {
		return models.Table{}, fmt.Errorf("Table %s not found in data model", params.TriggerTableName)
	}

	currentTable := firstTable
	for _, linkName := range params.Path {
		link, ok := currentTable.LinksToSingle[linkName]
		if !ok {
			return models.Table{}, fmt.Errorf("No link with name %s: %w", linkName, operators.ErrDbReadInconsistentWithDataModel)
		}
		nextTable, ok := params.DataModel.Tables[link.LinkedTableName]
		if !ok {
			return models.Table{}, fmt.Errorf("No table with name %s: %w", link.LinkedTableName, operators.ErrDbReadInconsistentWithDataModel)
		}

		currentTable = nextTable
	}
	return currentTable, nil
}
