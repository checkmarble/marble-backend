package pg_repository

import (
	"errors"
	"fmt"
	"marble/marble-backend/app"
	"testing"

	"github.com/Masterminds/squirrel"
	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pashagolub/pgxmock/v2"
)

type TestCase struct {
	name           string
	readParams     app.DbFieldReadParams
	expectedQuery  string
	expectedParams []interface{}
	expectedOutput interface{}
}

func TestReadRows(t *testing.T) {
	dataModel := app.DataModel{
		Tables: map[string]app.Table{
			"transactions": {
				Name: "transactions",
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
					"accounts": {
						LinkedTableName: "accounts",
						ParentFieldName: "object_id",
						ChildFieldName:  "account_id",
					},
				},
			},
			"accounts": {
				Name: "accounts",
				Fields: map[string]app.Field{
					"object_id": {
						DataType: app.String,
					},
					"updated_at":   {DataType: app.Timestamp},
					"status":       {DataType: app.String},
					"is_validated": {DataType: app.Bool},
				},
				LinksToSingle: map[string]app.LinkToSingle{},
			},
		},
	}
	payload := app.Payload{TableName: "transactions", Data: map[string]interface{}{"object_id": "1234"}}
	param := []interface{}{"1234"}
	cases := []TestCase{
		{

			name:           "Direct table read",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions"}, FieldName: "isValidated", DataModel: dataModel, Payload: payload},
			expectedQuery:  "SELECT transactions.isValidated FROM transactions WHERE transactions.object_id = $1",
			expectedParams: param,
			expectedOutput: pgtype.Bool{Bool: true, Valid: true},
		},
		{
			name:           "Table read with join - bool",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions", "accounts"}, FieldName: "isValidated", DataModel: dataModel, Payload: payload},
			expectedQuery:  "SELECT accounts.isValidated FROM transactions JOIN accounts ON transactions.account_id = accounts.object_id WHERE transactions.object_id = $1",
			expectedParams: param,
			expectedOutput: pgtype.Bool{Bool: true, Valid: true},
		},
		{
			name:           "Table read with join - string",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions", "accounts"}, FieldName: "status", DataModel: dataModel, Payload: payload},
			expectedQuery:  "SELECT accounts.status FROM transactions JOIN accounts ON transactions.account_id = accounts.object_id WHERE transactions.object_id = $1",
			expectedParams: param,
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

			val, err := repo.GetDbField(example.readParams)

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unfulfilled expectations: %s", err)
			}
			if !cmp.Equal(val, example.expectedOutput) {
				t.Errorf("Expected %v, got %v", example.expectedOutput, val)
			}

		})

	}

}

func TestNoRowsRead(t *testing.T) {
	dataModel := app.DataModel{
		Tables: map[string]app.Table{
			"transactions": {
				Name: "transactions",
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
					"accounts": {
						LinkedTableName: "accounts",
						ParentFieldName: "object_id",
						ChildFieldName:  "account_id",
					},
				},
			},
			"accounts": {
				Name: "accounts",
				Fields: map[string]app.Field{
					"object_id": {
						DataType: app.String,
					},
					"updated_at":   {DataType: app.Timestamp},
					"status":       {DataType: app.String},
					"is_validated": {DataType: app.Bool},
				},
				LinksToSingle: map[string]app.LinkToSingle{},
			},
		},
	}
	payload := app.Payload{TableName: "transactions", Data: map[string]interface{}{"object_id": "1234"}}
	param := []interface{}{"1234"}
	cases := []TestCase{
		{

			name:           "Direct table read",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions"}, FieldName: "isValidated", DataModel: dataModel, Payload: payload},
			expectedQuery:  "SELECT transactions.isValidated FROM transactions WHERE transactions.object_id = $1",
			expectedParams: param,
			expectedOutput: pgtype.Bool{Bool: true, Valid: true},
		},
		{
			name:           "Table read with join - bool",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions", "accounts"}, FieldName: "isValidated", DataModel: dataModel, Payload: payload},
			expectedQuery:  "SELECT accounts.isValidated FROM transactions JOIN accounts ON transactions.account_id = accounts.object_id WHERE transactions.object_id = $1",
			expectedParams: param,
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

			_, err = repo.GetDbField(example.readParams)
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
