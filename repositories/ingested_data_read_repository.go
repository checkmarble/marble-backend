package repositories

import (
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/operators"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type IngestedDataReadRepository interface {
	GetDbField(transaction Transaction, readParams models.DbFieldReadParams) (any, error)
	ListAllObjectsFromTable(transaction Transaction, table models.Table) ([]models.ClientObject, error)
}

type IngestedDataReadRepositoryImpl struct {
	queryBuilder squirrel.StatementBuilderType
}

func (repo *IngestedDataReadRepositoryImpl) GetDbField(transaction Transaction, readParams models.DbFieldReadParams) (any, error) {
	tx := adaptClientDatabaseTransaction(transaction)

	if len(readParams.Path) == 0 {
		return nil, fmt.Errorf("Path is empty: %w", operators.ErrDbReadInconsistentWithDataModel)
	}
	row, err := repo.queryDbForField(tx, readParams)
	if err != nil {
		return nil, fmt.Errorf("Error while building query for DB field: %w", err)
	}

	var output any
	err = row.Scan(&output)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("No rows scanned while reading DB: %w", operators.OperatorNoRowsReadInDbError)
	} else if err != nil {
		return nil, err
	}
	return output, nil
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

func getBaseObjectIdFromPayload(payload models.PayloadReader) (string, error) {
	baseObjectIdAny, _ := payload.ReadFieldFromPayload("object_id")
	if baseObjectIdAny == nil {
		return "", fmt.Errorf("object_id in payload is null") // should not happen, as per input validation
	}
	baseObjectId, ok := baseObjectIdAny.(string)
	if !ok {
		return "", fmt.Errorf("object_id in payload is not a string") // should not happen, as per input validation
	}

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

func (repo *IngestedDataReadRepositoryImpl) ListAllObjectsFromTable(transaction Transaction, table models.Table) ([]models.ClientObject, error) {
	tx := adaptClientDatabaseTransaction(transaction)

	columnNames := make([]string, len(table.Fields))
	i := 0
	for _, field := range table.Fields {
		columnNames[i] = string(field.Name)
		i++
	}

	objectsAsMap, err := queryWithDynamicColumnList(tx, string(table.Name), columnNames, repo.queryBuilder)
	if err != nil {
		return nil, err
	}

	output := make([]models.ClientObject, len(objectsAsMap))
	for i, objectAsMap := range objectsAsMap {
		object := models.ClientObject{
			TableName: table.Name,
			Data:      objectAsMap,
		}
		output[i] = object
	}

	return output, nil
}

func queryWithDynamicColumnList(tx TransactionPostgres, qualifiedTableName string, columnNames []string, queryBuilder squirrel.StatementBuilderType) ([]map[string]any, error) {
	sql, args, err := queryBuilder.
		Select(columnNames...).
		From(qualifiedTableName).
		Where(rowIsValid(qualifiedTableName)).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("Error while building SQL query: %w", err)
	}

	rows, err := tx.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("Error while querying DB: %w", err)
	}
	defer rows.Close()
	output := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columnNames))
		referencesToValues := make([]any, 0)
		for i := range values {
			referencesToValues = append(referencesToValues, &values[i])
		}

		err = rows.Scan(referencesToValues...)
		if err != nil {
			return nil, fmt.Errorf("Error while scanning row: %w", err)
		}

		objectAsMap := make(map[string]any)
		for i, columnName := range columnNames {
			objectAsMap[columnName] = values[i]
		}
		output = append(output, objectAsMap)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("Error while iterating over rows: %w", err)
	}

	return output, nil
}
