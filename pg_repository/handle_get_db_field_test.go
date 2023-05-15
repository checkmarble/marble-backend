package pg_repository

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"

	"marble/marble-backend/app"
)

func TestReadFromDb(t *testing.T) {
	transactions := app.Table{
		Name: "transactions",
		Fields: map[app.FieldName]app.Field{
			"object_id": {
				DataType: app.String,
			},
			"updated_at":  {DataType: app.Timestamp},
			"value":       {DataType: app.Float},
			"title":       {DataType: app.String},
			"accounts_id": {DataType: app.String},
		},
		LinksToSingle: map[app.LinkName]app.LinkToSingle{
			"accountss": {
				LinkedTableName: "accountss",
				ParentFieldName: "object_id",
				ChildFieldName:  "accounts_id",
			},
		},
	}
	accountss := app.Table{
		Name: "accountss",
		Fields: map[app.FieldName]app.Field{
			"object_id": {
				DataType: app.String,
			},
			"updated_at": {DataType: app.Timestamp},
			"name":       {DataType: app.String},
			"balance":    {DataType: app.Float},
			"company_id": {DataType: app.String},
		},
		LinksToSingle: map[app.LinkName]app.LinkToSingle{
			"companies": {
				LinkedTableName: "companies",
				ParentFieldName: "object_id",
				ChildFieldName:  "company_id",
			},
		},
	}
	companies := app.Table{
		Name: "companies",
		Fields: map[app.FieldName]app.Field{
			"object_id": {
				DataType: app.String,
			},
			"updated_at": {DataType: app.Timestamp},
			"name":       {DataType: app.String},
		},
		LinksToSingle: map[app.LinkName]app.LinkToSingle{},
	}
	dataModel := app.DataModel{
		Tables: map[app.TableName]app.Table{
			"transactions": transactions,
			"accountss":    accountss,
			"companies":    companies,
		},
	}
	ctx := context.Background()
	transactionId := globalTestParams.testIds["TransactionId"]
	payload, err := app.ParseToDataModelObject(ctx, transactions, []byte(fmt.Sprintf(`{"object_id": "%s", "updated_at": "2021-01-01T00:00:00Z"}`, transactionId)))
	if err != nil {
		t.Fatalf("Could not parse payload: %s", err)
	}
	payloadNotInDB, err := app.ParseToDataModelObject(ctx, transactions, []byte(fmt.Sprintf(`{"object_id": "%s", "updated_at": "2021-01-01T00:00:00Z"}`, "unknown transactionId")))
	if err != nil {
		t.Fatalf("Could not parse payload: %s", err)
	}

	type testCase struct {
		name           string
		readParams     app.DbFieldReadParams
		expectedOutput interface{}
		expectedError  error
	}

	cases := []testCase{
		{
			name:           "Read string field from DB with one join",
			readParams:     app.DbFieldReadParams{TriggerTableName: app.TableName("transactions"), Path: []app.LinkName{"accountss"}, FieldName: "name", DataModel: dataModel, Payload: payload},
			expectedOutput: pgtype.Text{String: "SHINE", Valid: true},
			expectedError:  nil,
		},
		{
			name:           "Read string field from DB with two joins",
			readParams:     app.DbFieldReadParams{TriggerTableName: app.TableName("transactions"), Path: []app.LinkName{"accountss", "companies"}, FieldName: "name", DataModel: dataModel, Payload: payload},
			expectedOutput: pgtype.Text{String: "Test company 1", Valid: true},
			expectedError:  nil,
		},
		{
			name:           "Read string field from DB, no line found",
			readParams:     app.DbFieldReadParams{TriggerTableName: app.TableName("transactions"), Path: []app.LinkName{"accountss"}, FieldName: "name", DataModel: dataModel, Payload: payloadNotInDB},
			expectedOutput: pgtype.Text{String: "", Valid: false},
			expectedError:  app.ErrNoRowsReadInDB,
		},
	}

	asserts := assert.New(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			val, err := globalTestParams.repository.GetDbField(context.Background(), c.readParams)

			if err != nil {
				asserts.True(errors.Is(err, c.expectedError), "Expected error %s, got %s", c.expectedError, err)
			}
			asserts.Equal(c.expectedOutput, val)

		})
	}
}
