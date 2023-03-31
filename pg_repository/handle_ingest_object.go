package pg_repository

import (
	"context"
	"fmt"
	"log"
	"marble/marble-backend/app"
)

func (r *PGRepository) IngestObject(payloadStructWithReader app.DynamicStructWithReader, table app.Table) (err error) {
	tx, err := r.db.Begin(context.Background())
	if err != nil {
		log.Printf("Error starting transaction: %s\n", err)
		return err
	}
	// Rollback is safe to call even if the tx is already closed, so if
	// the tx commits successfully, this is a no-op
	defer tx.Rollback(context.Background())

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

	log.Printf("args: %v\n", args)
	var createdObjectId string
	err = tx.QueryRow(context.TODO(), sql, args...).Scan(&createdObjectId)
	if err != nil {
		log.Printf("Error inserting object: %s\n", err)
		return err
	}
	log.Printf("Created object in db: type %s, id \"%s\"\n", table.Name, createdObjectId)

	err = tx.Commit(context.Background())
	if err != nil {
		log.Printf("Error committing transaction: %s\n", err)
		return err
	}
	return nil
}
