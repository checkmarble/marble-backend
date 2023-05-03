package pg_repository

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Masterminds/squirrel"
	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pashagolub/pgxmock/v2"

	"marble/marble-backend/app"
)

type MockedTestCase struct {
	name           string
	readParams     app.DbFieldReadParams
	expectedQuery  string
	expectedParams []interface{}
	expectedOutput interface{}
}

type LocalDbTestCase struct {
	name           string
	readParams     app.DbFieldReadParams
	expectedOutput interface{}
}

func TestReadFromDbWithDockerDb(t *testing.T) {
	transactions := app.Table{
		Name: "transactions_test",
		Fields: map[string]app.Field{
			"object_id": {
				DataType: app.String,
			},
			"updated_at":  {DataType: app.Timestamp},
			"value":       {DataType: app.Float},
			"isValidated": {DataType: app.Bool},
			"account_id":  {DataType: app.String},
		},
		LinksToSingle: map[string]app.LinkToSingle{
			"bank_accounts_test": {
				LinkedTableName: "bank_accounts_test",
				ParentFieldName: "object_id",
				ChildFieldName:  "account_id",
			},
		},
	}
	bank_accounts := app.Table{
		Name: "bank_accounts_test",
		Fields: map[string]app.Field{
			"object_id": {
				DataType: app.String,
			},
			"updated_at":   {DataType: app.Timestamp},
			"status":       {DataType: app.String},
			"is_validated": {DataType: app.Bool},
		},
		LinksToSingle: map[string]app.LinkToSingle{},
	}
	dataModel := app.DataModel{
		Tables: map[string]app.Table{
			"transactions_test":  transactions,
			"bank_accounts_test": bank_accounts,
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

	cases := []MockedTestCase{
		{
			name:           "Read boolean field from DB without join",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions_test"}, FieldName: "isValidated", DataModel: dataModel, Payload: *payload},
			expectedOutput: pgtype.Bool{Bool: true, Valid: true},
		},
		{
			name:           "Read float field from DB without join",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions_test"}, FieldName: "value", DataModel: dataModel, Payload: *payload},
			expectedOutput: pgtype.Float8{Float64: 10, Valid: true},
		},
		{
			name:           "Read null float field from DB without join",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions_test"}, FieldName: "value", DataModel: dataModel, Payload: *payloadNotInDB},
			expectedOutput: pgtype.Float8{Float64: 0, Valid: false},
		},
		{
			name:           "Read string field from DB with join",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions_test", "bank_accounts_test"}, FieldName: "status", DataModel: dataModel, Payload: *payload},
			expectedOutput: pgtype.Text{String: "VALIDATED", Valid: true},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			val, err := TestRepo.GetDbField(context.Background(), c.readParams)
			if err != nil {
				t.Errorf("Could not read field from DB: %s", err)
			}

			if !cmp.Equal(val, c.expectedOutput) {
				t.Errorf("Expected %v, got %v", c.expectedOutput, val)
			}
		})
	}

}

func TestReadRowsWithMockDb(t *testing.T) {
	transactions := app.Table{
		Name: "transactions_test",
		Fields: map[string]app.Field{
			"object_id": {
				DataType: app.String,
			},
			"updated_at":  {DataType: app.Timestamp},
			"value":       {DataType: app.Float},
			"isValidated": {DataType: app.Bool},
			"account_id":  {DataType: app.String},
		},
		LinksToSingle: map[string]app.LinkToSingle{
			"bank_accounts_test": {
				LinkedTableName: "bank_accounts_test",
				ParentFieldName: "object_id",
				ChildFieldName:  "account_id",
			},
		},
	}
	bank_accounts := app.Table{
		Name: "bank_accounts_test",
		Fields: map[string]app.Field{
			"object_id": {
				DataType: app.String,
			},
			"updated_at":   {DataType: app.Timestamp},
			"status":       {DataType: app.String},
			"is_validated": {DataType: app.Bool},
		},
		LinksToSingle: map[string]app.LinkToSingle{},
	}
	dataModel := app.DataModel{
		Tables: map[string]app.Table{
			"transactions_test":  transactions,
			"bank_accounts_test": bank_accounts,
		}}

	ctx := context.Background()
	transactionId1 := globalTestParams.testIds["TransactionId1"]
	payload, err := app.ParseToDataModelObject(ctx, transactions, []byte(fmt.Sprintf(`{"object_id": "%s", "updated_at": "2021-01-01T00:00:00Z"}`, transactionId1)))
	if err != nil {
		t.Fatalf("Could not parse payload: %s", err)
	}
	cases := []MockedTestCase{
		{

			name:           "Direct table read",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions_test"}, FieldName: "isValidated", DataModel: dataModel, Payload: *payload},
			expectedQuery:  "SELECT transactions.isValidated FROM transactions WHERE transactions.object_id = $1 AND transactions.valid_until = $2",
			expectedParams: []interface{}{transactionId1, "Infinity"},
			expectedOutput: pgtype.Bool{Bool: true, Valid: true},
		},
		{
			name:           "Table read with join - bool",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions_test", "bank_accounts_test"}, FieldName: "isValidated", DataModel: dataModel, Payload: *payload},
			expectedQuery:  "SELECT bank_accounts.isValidated FROM transactions JOIN bank_accounts ON transactions.account_id = bank_accounts.object_id WHERE bank_accounts.valid_until = $1 AND transactions.object_id = $2 AND transactions.valid_until = $3",
			expectedParams: []interface{}{"Infinity", transactionId1, "Infinity"},
			expectedOutput: pgtype.Bool{Bool: true, Valid: true},
		},
		{
			name:           "Table read with join - string",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions_test", "bank_accounts_test"}, FieldName: "status", DataModel: dataModel, Payload: *payload},
			expectedQuery:  "SELECT bank_accounts.status FROM transactions JOIN bank_accounts ON transactions.account_id = bank_accounts.object_id WHERE bank_accounts.valid_until = $1 AND transactions.object_id = $2 AND transactions.valid_until = $3",
			expectedParams: []interface{}{"Infinity", transactionId1, "Infinity"},
			expectedOutput: pgtype.Text{String: "VALIDATED", Valid: true},
		},
	}

	for _, example := range cases {
		t.Run(example.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool(pgxmock.QueryMatcherOption(pgxmock.QueryMatcherEqual), pgxmock.MonitorPingsOption(true))
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()

			rows := mock.NewRows([]string{example.readParams.FieldName}).AddRow(example.expectedOutput)
			mock.ExpectQuery(example.expectedQuery).WithArgs(example.expectedParams...).WillReturnRows(rows)

			repo := PGRepository{db: mock, queryBuilder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}

			val, err := repo.GetDbField(context.Background(), example.readParams)

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unfulfilled expectations: %s", err)
			}
			if !cmp.Equal(val, example.expectedOutput) {
				t.Errorf("Expected %v, got %v", example.expectedOutput, val)
			}

		})

	}

}

func TestNoRowsReadWithMockDb(t *testing.T) {
	transactions := app.Table{
		Name: "transactions_test",
		Fields: map[string]app.Field{
			"object_id": {
				DataType: app.String,
			},
			"updated_at":  {DataType: app.Timestamp},
			"value":       {DataType: app.Float},
			"isValidated": {DataType: app.Bool},
			"account_id":  {DataType: app.String},
		},
		LinksToSingle: map[string]app.LinkToSingle{
			"bank_accounts_test": {
				LinkedTableName: "bank_accounts_test",
				ParentFieldName: "object_id",
				ChildFieldName:  "account_id",
			},
		},
	}
	bank_accounts := app.Table{
		Name: "bank_accounts_test",
		Fields: map[string]app.Field{
			"object_id": {
				DataType: app.String,
			},
			"updated_at":   {DataType: app.Timestamp},
			"status":       {DataType: app.String},
			"is_validated": {DataType: app.Bool},
		},
		LinksToSingle: map[string]app.LinkToSingle{},
	}
	dataModel := app.DataModel{
		Tables: map[string]app.Table{
			"transactions_test":  transactions,
			"bank_accounts_test": bank_accounts,
		}}
	ctx := context.Background()
	transactionId1 := globalTestParams.testIds["TransactionId1"]
	payload, err := app.ParseToDataModelObject(ctx, transactions, []byte(fmt.Sprintf(`{"object_id": "%s", "updated_at": "2021-01-01T00:00:00Z"}`, transactionId1)))
	if err != nil {
		t.Fatalf("Could not parse payload: %s", err)
	}
	cases := []MockedTestCase{
		{

			name:           "Direct table read",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions_test"}, FieldName: "isValidated", DataModel: dataModel, Payload: *payload},
			expectedQuery:  "SELECT transactions.isValidated FROM transactions WHERE transactions.object_id = $1 AND transactions.valid_until = $2",
			expectedParams: []interface{}{transactionId1, "Infinity"},
			expectedOutput: pgtype.Bool{Bool: true, Valid: true},
		},
		{
			name:           "Table read with join - bool",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions_test", "bank_accounts_test"}, FieldName: "isValidated", DataModel: dataModel, Payload: *payload},
			expectedQuery:  "SELECT bank_accounts.isValidated FROM transactions JOIN bank_accounts ON transactions.account_id = bank_accounts.object_id WHERE bank_accounts.valid_until = $1 AND transactions.object_id = $2 AND transactions.valid_until = $3",
			expectedParams: []interface{}{"Infinity", transactionId1, "Infinity"},
			expectedOutput: pgtype.Bool{Bool: true, Valid: true},
		},
	}

	for _, example := range cases {
		t.Run(example.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool(pgxmock.QueryMatcherOption(pgxmock.QueryMatcherEqual), pgxmock.MonitorPingsOption(true))
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()

			mock.ExpectQuery(example.expectedQuery).WithArgs(example.expectedParams...).WillReturnError(pgx.ErrNoRows)
			repo := PGRepository{db: mock, queryBuilder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}

			_, err = repo.GetDbField(context.Background(), example.readParams)
			if err != nil {
				fmt.Printf("Error: %s", err)
				if errors.Is(err, app.ErrNoRowsReadInDB) {
					fmt.Println("No rows found, as expected")
				} else {
					t.Errorf("Expected no error, got %v", err)
				}
			}

		})

	}

}
