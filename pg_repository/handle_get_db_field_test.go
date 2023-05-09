package pg_repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5/pgtype"

	"marble/marble-backend/app"
)

func TestReadFromDbWithDockerDb(t *testing.T) {
	transactions := app.Table{
		Name: "transactions",
		Fields: map[string]app.Field{
			"object_id": {
				DataType: app.String,
			},
			"updated_at":      {DataType: app.Timestamp},
			"value":           {DataType: app.Float},
			"title":           {DataType: app.String},
			"bank_account_id": {DataType: app.String},
		},
		LinksToSingle: map[string]app.LinkToSingle{
			"bank_accounts": {
				LinkedTableName: "bank_accounts",
				ParentFieldName: "object_id",
				ChildFieldName:  "bank_account_id",
			},
		},
	}
	bank_accounts := app.Table{
		Name: "bank_accounts",
		Fields: map[string]app.Field{
			"object_id": {
				DataType: app.String,
			},
			"updated_at": {DataType: app.Timestamp},
			"name":       {DataType: app.String},
			"balance":    {DataType: app.Float},
		},
		LinksToSingle: map[string]app.LinkToSingle{},
	}
	dataModel := app.DataModel{
		Tables: map[string]app.Table{
			"transactions":  transactions,
			"bank_accounts": bank_accounts,
		},
	}
	ctx := context.Background()
	transactionId1 := globalTestParams.testIds["TransactionId1"]
	transactionId2 := globalTestParams.testIds["TransactionId2"]
	payload, err := app.ParseToDataModelObject(ctx, transactions, []byte(fmt.Sprintf(`{"object_id": "%s", "updated_at": "2021-01-01T00:00:00Z"}`, transactionId1)))
	if err != nil {
		t.Fatalf("Could not parse payload: %s", err)
	}
	payloadNotInDB, err := app.ParseToDataModelObject(ctx, transactions, []byte(fmt.Sprintf(`{"object_id": "%s", "updated_at": "2021-01-01T00:00:00Z"}`, transactionId2)))
	if err != nil {
		t.Fatalf("Could not parse payload: %s", err)
	}

	type localDbTestCase struct {
		name           string
		readParams     app.DbFieldReadParams
		expectedOutput interface{}
	}

	cases := []localDbTestCase{
		{
			name:           "Read boolean field from DB without join",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions"}, FieldName: "title", DataModel: dataModel, Payload: payload},
			expectedOutput: pgtype.Text{String: "AMAZON", Valid: true},
		},
		{
			name:           "Read float field from DB without join",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions"}, FieldName: "value", DataModel: dataModel, Payload: payload},
			expectedOutput: pgtype.Float8{Float64: 10, Valid: true},
		},
		{
			name:           "Read null float field from DB without join",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions"}, FieldName: "value", DataModel: dataModel, Payload: payloadNotInDB},
			expectedOutput: pgtype.Float8{Float64: 0, Valid: false},
		},
		{
			name:           "Read string field from DB with join",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions", "bank_accounts"}, FieldName: "name", DataModel: dataModel, Payload: payload},
			expectedOutput: pgtype.Text{String: "SHINE", Valid: true},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			val, err := globalTestParams.repository.GetDbField(context.Background(), c.readParams)
			if err != nil {
				t.Errorf("Could not read field from DB: %s", err)
			}

			if !cmp.Equal(val, c.expectedOutput) {
				t.Errorf("Expected %v, got %v", c.expectedOutput, val)
			}
		})
	}
}
