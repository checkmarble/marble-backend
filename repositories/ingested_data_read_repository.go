package repositories

import (
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type IngestedDataReadRepository interface {
	GetDbField(transaction Transaction, readParams models.DbFieldReadParams) (any, error)
	ListAllObjectsFromTable(transaction Transaction, table models.Table) ([]models.ClientObject, error)
	QueryAggregatedValue(transaction Transaction, tableName models.TableName, fieldName models.FieldName, aggregator ast.Aggregator, filters []ast.Filter) (any, error)
}

type IngestedDataReadRepositoryImpl struct{}

func (repo *IngestedDataReadRepositoryImpl) GetDbField(transaction Transaction, readParams models.DbFieldReadParams) (any, error) {
	tx := adaptClientDatabaseTransaction(transaction)

	if len(readParams.Path) == 0 {
		return nil, fmt.Errorf("path is empty: %w", models.BadParameterError)
	}
	row, err := repo.queryDbForField(tx, readParams)
	if err != nil {
		return nil, fmt.Errorf("error while building query for DB field: %w", err)
	}

	var output any
	err = row.Scan(&output)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("no rows scanned while reading DB: %w", models.NoRowsReadError)
	} else if err != nil {
		return nil, err
	}
	return output, nil
}

func (repo *IngestedDataReadRepositoryImpl) queryDbForField(tx TransactionPostgres, readParams models.DbFieldReadParams) (pgx.Row, error) {
	triggerTable, ok := readParams.DataModel.Tables[readParams.TriggerTableName]
	if !ok {
		return nil, fmt.Errorf("table %s not found in data model", readParams.TriggerTableName)
	}
	link, ok := triggerTable.LinksToSingle[readParams.Path[0]]
	if !ok {
		return nil, fmt.Errorf("no link with name %s: %w", readParams.Path[0], models.NotFoundError)
	}

	firstTableObjectId, err := getFirstTableObjectIdFromPayload(readParams.Payload, link.ChildFieldName)
	if err != nil {
		return nil, fmt.Errorf("error while getting first path table object id from payload: %w", err)
	}

	// "first table" is the first table reached starting from the trigger table and following the path
	firstTable, ok := readParams.DataModel.Tables[link.LinkedTableName]
	if !ok {
		return nil, fmt.Errorf("no table with name %s: %w", link.LinkedTableName, models.NotFoundError)
	}
	// "last table" is the last table reached starting from the trigger table and following the path, from which the field is selected
	lastTable, err := getLastTableFromPath(readParams, firstTable)
	if err != nil {
		return nil, err
	}

	firstTableName := tableNameWithSchema(tx, firstTable.Name)
	lastTableName := tableNameWithSchema(tx, lastTable.Name)

	// setup the end table we read the field from, the beginning table we join from, and relevant filters on the latter
	query := NewQueryBuilder().
		Select(fmt.Sprintf("%s.%s", lastTableName, readParams.FieldName)).
		From(firstTableName).
		Where(squirrel.Eq{fmt.Sprintf("%s.object_id", firstTableName): firstTableObjectId}).
		Where(rowIsValid(firstTableName))

	query, err = addJoinsOnIntermediateTables(tx, query, readParams, firstTable)
	if err != nil {
		return nil, err
	}

	sql, args, err := query.ToSql()

	if err != nil {
		return nil, fmt.Errorf("error while building SQL query: %w", err)
	}

	row := tx.exec.QueryRow(tx.ctx, sql, args...)
	return row, nil
}

func getFirstTableObjectIdFromPayload(payload models.PayloadReader, fieldName models.FieldName) (string, error) {
	parentObjectIdItf, _ := payload.ReadFieldFromPayload(fieldName)
	if parentObjectIdItf == nil {
		return "", fmt.Errorf("%s in payload is null", fieldName) // should not happen, as per input validation
	}
	parentObjectId, ok := parentObjectIdItf.(string)
	if !ok {
		return "", fmt.Errorf("%s in payload is not a string", fieldName) // should not happen, as per input validation
	}

	return parentObjectId, nil
}

func addJoinsOnIntermediateTables(tx TransactionPostgres, query squirrel.SelectBuilder, readParams models.DbFieldReadParams, firstTable models.Table) (squirrel.SelectBuilder, error) {
	currentTable := firstTable
	// ignore the first element of the path, as it is the starting table of the query
	for _, linkName := range readParams.Path[1:] {
		link, ok := currentTable.LinksToSingle[linkName]
		if !ok {
			return squirrel.SelectBuilder{}, fmt.Errorf("no link with name %s on table %s: %w", linkName, currentTable.Name, models.NotFoundError)
		}
		nextTable, ok := readParams.DataModel.Tables[link.LinkedTableName]
		if !ok {
			return squirrel.SelectBuilder{}, fmt.Errorf("no table with name %s: %w", link.LinkedTableName, models.NotFoundError)
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

func getLastTableFromPath(params models.DbFieldReadParams, firstTable models.Table) (models.Table, error) {
	currentTable := firstTable
	// ignore the first element of the path, as it is the starting table of the query
	for _, linkName := range params.Path[1:] {
		link, ok := currentTable.LinksToSingle[linkName]
		if !ok {
			return models.Table{}, fmt.Errorf("no link with name %s: %w", linkName, models.NotFoundError)
		}
		nextTable, ok := params.DataModel.Tables[link.LinkedTableName]
		if !ok {
			return models.Table{}, fmt.Errorf("no table with name %s: %w", link.LinkedTableName, models.NotFoundError)
		}

		currentTable = nextTable
	}
	return currentTable, nil
}

func (repo *IngestedDataReadRepositoryImpl) ListAllObjectsFromTable(transaction Transaction, table models.Table) ([]models.ClientObject, error) {
	tx := adaptClientDatabaseTransaction(transaction)

	columnNames := make([]string, len(table.Fields))
	i := 0
	for fieldName := range table.Fields {
		columnNames[i] = string(fieldName)
		i++
	}

	objectsAsMap, err := queryWithDynamicColumnList(tx, tableNameWithSchema(tx, table.Name), columnNames)
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

func queryWithDynamicColumnList(tx TransactionPostgres, qualifiedTableName string, columnNames []string) ([]map[string]any, error) {
	sql, args, err := NewQueryBuilder().
		Select(columnNames...).
		From(qualifiedTableName).
		Where(rowIsValid(qualifiedTableName)).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("error while building SQL query: %w", err)
	}
	rows, err := tx.exec.Query(tx.ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("error while querying DB: %w", err)
	}
	defer rows.Close()
	output := make([]map[string]any, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("error while fetching rows: %w", err)
		}

		objectAsMap := make(map[string]any)
		for i, columnName := range columnNames {
			objectAsMap[columnName] = values[i]
		}
		output = append(output, objectAsMap)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error while iterating over rows: %w", err)
	}

	return output, nil
}

func (repo *IngestedDataReadRepositoryImpl) QueryAggregatedValue(transaction Transaction, tableName models.TableName, fieldName models.FieldName, aggregator ast.Aggregator, filters []ast.Filter) (any, error) {
	tx := adaptClientDatabaseTransaction(transaction)

	var selectExpression string
	if aggregator == ast.AGGREGATOR_COUNT_DISTINCT {
		selectExpression = fmt.Sprintf("COUNT(DISTINCT %s)", fieldName)
	} else {
		selectExpression = fmt.Sprintf("%s(%s)", aggregator, fieldName)
	}

	qualifiedTableName := tableNameWithSchema(tx, tableName)

	query := NewQueryBuilder().
		Select(selectExpression).
		From(qualifiedTableName).
		Where(rowIsValid(qualifiedTableName))

	var err error
	for _, filter := range filters {
		query, err = addConditionForOperator(query, qualifiedTableName, filter.FieldName, filter.Operator, filter.Value)
		if err != nil {
			return nil, err
		}
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("error while building SQL query: %w", err)
	}
	var result any
	err = tx.exec.QueryRow(tx.ctx, sql, args...).Scan(&result)
	if err != nil {
		return nil, fmt.Errorf("error while querying DB: %w", err)
	}

	return result, nil
}

func addConditionForOperator(query squirrel.SelectBuilder, tableName string, fieldName string, operator ast.FilterOperator, value any) (squirrel.SelectBuilder, error) {
	switch operator {
	case ast.FILTER_EQUAL:
		return query.Where(squirrel.Eq{fmt.Sprintf("%s.%s", tableName, fieldName): value}), nil
	case ast.FILTER_NOT_EQUAL:
		return query.Where(squirrel.NotEq{fmt.Sprintf("%s.%s", tableName, fieldName): value}), nil
	case ast.FILTER_GREATER:
		return query.Where(squirrel.Gt{fmt.Sprintf("%s.%s", tableName, fieldName): value}), nil
	case ast.FILTER_GREATER_OR_EQUAL:
		return query.Where(squirrel.GtOrEq{fmt.Sprintf("%s.%s", tableName, fieldName): value}), nil
	case ast.FILTER_LESSER:
		return query.Where(squirrel.Lt{fmt.Sprintf("%s.%s", tableName, fieldName): value}), nil
	case ast.FILTER_LESSER_OR_EQUAL:
		return query.Where(squirrel.LtOrEq{fmt.Sprintf("%s.%s", tableName, fieldName): value}), nil
	default:
		return query, fmt.Errorf("unknown operator %s: %w", operator, models.BadParameterError)
	}
}
