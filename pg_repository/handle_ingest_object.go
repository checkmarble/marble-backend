package pg_repository

import (
	"context"
	"fmt"
	"log"
	"marble/marble-backend/app"
)

func (r *PGRepository) IngestObject(ctx context.Context, payloadStructWithReader app.DynamicStructWithReader, table app.Table) (err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		log.Printf("Error starting transaction: %s\n", err)
		return err
	}
	// Rollback is safe to call even if the tx is already closed, so if
	// the tx commits successfully, this is a no-op
	defer tx.Rollback(ctx)

	columnNamesSlice := make([]string, len(table.Fields))
	valuesNumberSlice := make([]string, len(table.Fields))
	values := make([]interface{}, len(table.Fields))
	i := 0
	for fieldName := range table.Fields {
		columnNamesSlice[i] = fieldName
		valuesNumberSlice[i] = fmt.Sprintf("$%d", i+1)
		values[i] = payloadStructWithReader.ReadFieldFromDynamicStruct(fieldName)
		i++
	}

	sql, args, err := r.queryBuilder.Insert(table.Name).Columns(columnNamesSlice...).Values(values...).Suffix("RETURNING \"id\"").ToSql()
	if err != nil {
		log.Printf("Error building the query: %s\n", err)
		return err
	}

	log.Printf("args: %v\n", args)
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
