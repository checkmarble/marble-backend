package pg_repository

import (
	"context"
	"errors"
	"log"
	"marble/marble-backend/app"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

func generateInsertValues(table app.Table, payloadStructWithReader app.DynamicStructWithReader) (columnNames []string, values []interface{}) {
	nbFields := len(table.Fields)
	columnNames = make([]string, nbFields)
	values = make([]interface{}, nbFields)
	i := 0
	for fieldName := range table.Fields {
		columnNames[i] = fieldName
		values[i] = payloadStructWithReader.ReadFieldFromDynamicStruct(fieldName)
		i++
	}
	return columnNames, values
}

func updateExistingVersionIfPresent(
	ctx context.Context,
	queryBuilder squirrel.StatementBuilderType,
	tx pgx.Tx,
	payloadStructWithReader app.DynamicStructWithReader,
	table app.Table) (err error) {

	sql, args, err := queryBuilder.
		Select("id").
		From(table.Name).
		Where(squirrel.Eq{"object_id": payloadStructWithReader.ReadFieldFromDynamicStruct("object_id")}).
		Where(squirrel.Eq{"valid_until": "Infinity"}).
		ToSql()
	if err != nil {
		return err
	}

	var id string
	err = tx.QueryRow(ctx, sql, args...).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		} else {
			return err
		}
	}

	sql, args, err = queryBuilder.
		Update(table.Name).
		Set("valid_until", "now()").
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}

	return nil
}

func (r *PGRepository) IngestObject(ctx context.Context, payloadStructWithReader app.DynamicStructWithReader, table app.Table) (err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		log.Printf("Error starting transaction: %s\n", err)
		return err
	}
	defer tx.Rollback(ctx)

	err = updateExistingVersionIfPresent(ctx, r.queryBuilder, tx, payloadStructWithReader, table)
	if err != nil {
		log.Printf("Error updating existing version: %s\n", err)
		return err
	}

	columnNames, values := generateInsertValues(table, payloadStructWithReader)
	sql, args, err := r.queryBuilder.Insert(table.Name).Columns(columnNames...).Values(values...).Suffix("RETURNING \"id\"").ToSql()
	if err != nil {
		log.Printf("Error building the query: %s\n", err)
		return err
	}

	var createdObjectID string
	err = tx.QueryRow(ctx, sql, args...).Scan(&createdObjectID)
	if err != nil {
		log.Printf("Error inserting object: %s\n", err)
		return err
	}
	log.Printf("Created object in db: type %s, id \"%s\"\n", table.Name, createdObjectID)

	err = tx.Commit(ctx)
	if err != nil {
		log.Printf("Error committing transaction: %s\n", err)
		return err
	}
	return nil
}
