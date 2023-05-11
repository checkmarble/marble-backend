package pg_repository

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/app"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"golang.org/x/exp/slog"
)

func generateInsertValues(table app.Table, payloadStructWithReader app.DynamicStructWithReader) (columnNames []string, values []interface{}) {
	nbFields := len(table.Fields)
	columnNames = make([]string, nbFields)
	values = make([]interface{}, nbFields)
	i := 0
	for fieldName := range table.Fields {
		columnNames[i] = string(fieldName)
		values[i] = payloadStructWithReader.ReadFieldFromDynamicStruct(fieldName)
		i++
	}
	return columnNames, values
}

func updateExistingVersionIfPresent(
	ctx context.Context,
	queryBuilder sq.StatementBuilderType,
	tx pgx.Tx,
	payloadStructWithReader app.DynamicStructWithReader,
	table app.Table) (err error) {

	object_id := payloadStructWithReader.ReadFieldFromDynamicStruct("object_id")
	sql, args, err := queryBuilder.
		Select("id").
		From(string(table.Name)).
		Where(sq.Eq{"object_id": object_id}).
		Where(sq.Eq{"valid_until": "Infinity"}).
		ToSql()
	if err != nil {
		return err
	}

	var id string
	err = tx.QueryRow(ctx, sql, args...).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}

	sql, args, err = queryBuilder.
		Update(string(table.Name)).
		Set("valid_until", "now()").
		Where(sq.Eq{"id": id}).
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

func (r *PGRepository) IngestObject(ctx context.Context, payloadStructWithReader app.DynamicStructWithReader, table app.Table, logger *slog.Logger) (err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Error starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	err = updateExistingVersionIfPresent(ctx, r.queryBuilder, tx, payloadStructWithReader, table)
	if err != nil {
		return fmt.Errorf("Error updating existing version: %w", err)
	}

	columnNames, values := generateInsertValues(table, payloadStructWithReader)
	sql, args, err := r.queryBuilder.Insert(string(table.Name)).Columns(columnNames...).Values(values...).Suffix("RETURNING \"id\"").ToSql()
	if err != nil {
		return fmt.Errorf("Error building the query: %w", err)
	}

	var createdObjectID string
	err = tx.QueryRow(ctx, sql, args...).Scan(&createdObjectID)
	if err != nil {
		return fmt.Errorf("Error inserting object: %w", err)
	}
	logger.InfoCtx(ctx, "Created object in db", slog.String("type", string(table.Name)), slog.String("object_id", createdObjectID))

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("Error committing transaction: %w", err)
	}
	return nil
}
