package pg_repository

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/app/operators"
	"marble/marble-backend/models"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (rep *PGRepository) GetDbField(ctx context.Context, readParams models.DbFieldReadParams) (interface{}, error) {
	if len(readParams.Path) == 0 {
		return nil, fmt.Errorf("Path is empty: %w", operators.ErrDbReadInconsistentWithDataModel)
	}
	row, err := rep.queryDbForField(ctx, readParams)
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
		return returnVariable, fmt.Errorf("No rows scanned while reading DB: %w", models.OperatorNoRowsReadInDbError)
	} else if err != nil {
		return returnVariable, err
	}
	return returnVariable, nil
}

func (rep *PGRepository) queryDbForField(ctx context.Context, readParams models.DbFieldReadParams) (pgx.Row, error) {
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

	// setup the end table we read the field from, the beginning table we join from, and relevant filters on the latter
	query := rep.queryBuilder.
		Select(fmt.Sprintf("%s.%s", lastTable.Name, readParams.FieldName)).
		From(string(firstTable.Name)).
		Where(sq.Eq{fmt.Sprintf("%s.object_id", firstTable.Name): baseObjectId}).
		Where(rowIsValid(firstTable.Name))

	query, err = addJoinsOnIntermediateTables(query, readParams, firstTable)
	if err != nil {
		return nil, err
	}

	sql, args, err := query.ToSql()

	if err != nil {
		return nil, fmt.Errorf("Error while building SQL query: %w", err)
	}

	rows := rep.db.QueryRow(ctx, sql, args...)
	return rows, nil
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

func addJoinsOnIntermediateTables(query sq.SelectBuilder, readParams models.DbFieldReadParams, firstTable models.Table) (sq.SelectBuilder, error) {
	currentTable := firstTable
	for _, linkName := range readParams.Path {
		link, ok := currentTable.LinksToSingle[linkName]
		if !ok {
			return sq.SelectBuilder{}, fmt.Errorf("No link with name %s on table %s: %w", linkName, currentTable.Name, operators.ErrDbReadInconsistentWithDataModel)
		}
		nextTable, ok := readParams.DataModel.Tables[link.LinkedTableName]
		if !ok {
			return sq.SelectBuilder{}, fmt.Errorf("No table with name %s: %w", link.LinkedTableName, operators.ErrDbReadInconsistentWithDataModel)
		}

		joinClause := fmt.Sprintf("%s ON %s.%s = %s.%s", nextTable.Name, currentTable.Name, link.ChildFieldName, nextTable.Name, link.ParentFieldName)
		query = query.Join(joinClause).
			Where(rowIsValid(nextTable.Name))

		currentTable = nextTable
	}
	return query, nil
}

func rowIsValid(tableName models.TableName) sq.Eq {
	return sq.Eq{fmt.Sprintf("%s.valid_until", tableName): "Infinity"}
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
