package pg_repository

import (
	"context"
	"fmt"
	"log"
	"marble/marble-backend/app"
	"strings"
)

func (r *PGRepository) PayloadToDynamicStruct(payload app.IngestPayload, dataModel app.DataModel) (err error) {
	return nil
}

func (r *PGRepository) IngestObject(orgID string, ingestPayload app.IngestPayload) (err error) {
	dataModel, err := r.GetDataModel(orgID)
	if err != nil {
		log.Printf("Unable to find datamodel by orgId for ingestion: %v", err)
		return err
	}

	_ = r.PayloadToDynamicStruct(ingestPayload, dataModel)
	payloadStructWithReader, err := app.ParseToDataModelObject(dataModel, ingestPayload.ObjectBody, ingestPayload.ObjectType)
	if err != nil {
		log.Printf("Error while parsing struct in repository IngestObject: %v", err)
		return err
	}

	tx, err := r.db.Begin(context.Background())
	if err != nil {
		return err
	}
	// Rollback is safe to call even if the tx is already closed, so if
	// the tx commits successfully, this is a no-op
	defer tx.Rollback(context.Background())

	tables := dataModel.Tables
	table, ok := tables[ingestPayload.ObjectType]
	if !ok {
		return fmt.Errorf("table %s not found in data model", ingestPayload.ObjectType)
	}

	columnNamesSlice := make([]string, len(table.Fields))
	valuesNumberSlice := make([]string, len(table.Fields))
	values := make([]interface{}, len(table.Fields))
	i := 0
	for k := range table.Fields {
		columnNamesSlice[i] = k
		valuesNumberSlice[i] = fmt.Sprintf("$%d", i+1)
		values[i] = app.ReadFieldFromDynamicStruct(payloadStructWithReader, k)
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
	`, ingestPayload.ObjectType, columnNames, valuesNumbers)

	var createdObjectId string
	err = tx.QueryRow(context.TODO(), insertDecisionQueryString, values...,
	).Scan(&createdObjectId)

	fmt.Printf("Created object in db: type %s, id %s", ingestPayload.ObjectType, createdObjectId)
	if err != nil {
		return err
	}
	return nil
}
