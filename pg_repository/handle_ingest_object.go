package pg_repository

import (
	"context"
	"fmt"
	"marble/marble-backend/app"
	"strings"
)

func (r *PGRepository) IngestObject(payloadStructWithReader app.DynamicStructWithReader, table app.Table) (err error) {
	tx, err := r.db.Begin(context.Background())
	if err != nil {
		return err
	}
	// Rollback is safe to call even if the tx is already closed, so if
	// the tx commits successfully, this is a no-op
	defer tx.Rollback(context.Background())

	columnNamesSlice := make([]string, len(table.Fields))
	valuesNumberSlice := make([]string, len(table.Fields))
	values := make([]interface{}, len(table.Fields))
	i := 0
	for k := range table.Fields {
		columnNamesSlice[i] = k
		valuesNumberSlice[i] = fmt.Sprintf("$%d", i+1)
		values[i] = payloadStructWithReader.ReadFieldFromDynamicStruct(k)
		i++
	}

	columnNames := strings.Join(columnNamesSlice, ", ")
	valuesNumbers := strings.Join(valuesNumberSlice, ", ")
	// insert the decision
	insertDecisionQueryString := fmt.Sprintf(`
	INSERT INTO %s
	(%s)
	VALUES (%s)
	RETURNING "id";
	`, table.Name, columnNames, valuesNumbers)

	var createdObjectId string
	err = tx.QueryRow(context.TODO(), insertDecisionQueryString, values...,
	).Scan(&createdObjectId)

	fmt.Printf("Created object in db: type %s, id %s", table.Name, createdObjectId)
	if err != nil {
		return err
	}
	return nil
}
