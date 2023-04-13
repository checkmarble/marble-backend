package pg_repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
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

const testDbLifetime = 120

const (
	testUser     = "test_user"
	testPassword = "pwd"
	testHost     = "localhost"
	testDbName   = "test_db"
	testPort     = "5432"
)

var dbpool *pgxpool.Pool

func TestMain(m *testing.M) {
	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not construct pool: %s", err)
	}

	err = pool.Client.Ping()
	if err != nil {
		log.Fatalf("Could not connect to Docker: %s", err)
	}

	// pulls an image, creates a container based on it and runs it
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "15",
		Env: []string{
			fmt.Sprintf("POSTGRES_PASSWORD=%s", testPassword),
			fmt.Sprintf("POSTGRES_USER=%s", testUser),
			fmt.Sprintf("POSTGRES_DB=%s", testDbName),
			"listen_addresses = '*'",
		},
	}, func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	resource.Expire(testDbLifetime) // Tell docker to hard kill the container in testDbLifetime seconds

	pool.MaxWait = testDbLifetime * time.Second

	hostAndPort := resource.GetHostPort("5432/tcp") // docker container will bind to another port than 5432 if already taken
	databaseUrl := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", testUser, testPassword, hostAndPort, testDbName)
	dbpool, err = pgxpool.New(context.Background(), databaseUrl)
	if err != nil {
		log.Fatalf("Could not connect to database: %s", err)
	}

	if err = pool.Retry(func() error {
		log.Printf("DB connection pool created. Stats: %+v\n", dbpool.Stat())
		err = dbpool.Ping(context.Background())
		if err != nil {
			log.Printf("Could not ping database: %s", err)
			return err
		}
		return nil
	}); err != nil {
		log.Fatalf("Could not connect to db: %s", err)
	}

	createTablesSql := `
	CREATE SCHEMA testschema;

	GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA testschema TO test_user;

	ALTER DATABASE test_db
	SET search_path TO testschema,
	public;

	ALTER ROLE test_user
	SET search_path TO testschema,
	public;

	CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

	CREATE TABLE transactions(
		id uuid DEFAULT uuid_generate_v4(),
		object_id VARCHAR NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		account_id VARCHAR,
		title VARCHAR,
		value FLOAT,
		isValidated BOOLEAN,
		PRIMARY KEY(id)
	  );
	CREATE TABLE accounts(
		ID UUID DEFAULT uuid_generate_v4(),
		object_id VARCHAR NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		status VARCHAR,
		PRIMARY KEY(id)
	);

	INSERT INTO accounts (
		object_id,
		updated_at,
		status
	  )
	VALUES(
		'5c8a32f9-29c7-413c-91a8-1a363ef7e6b5',
		'2021-01-01T00:00:00Z',
		'VALIDATED'
	  );

	INSERT INTO transactions (
		object_id,
		account_id,
		updated_at,
		value,
		isValidated
	  )
	VALUES(
		'9283b948-a140-4993-9c41-d5475fda5671',
		'5c8a32f9-29c7-413c-91a8-1a363ef7e6b5',
		'2021-01-01T00:00:00Z',
		10,
		true
	  ),(
		'6d3a330d-7204-4561-b523-9fa0d518d184',
		'5c8a32f9-29c7-413c-91a8-1a363ef7e6b5',
		'2021-01-01T00:00:00Z',
		NULL,
		false
	  );
	  `

	if _, err := dbpool.Exec(context.Background(), createTablesSql); err != nil {
		log.Fatalf("Could not create tables: %s", err)
	}

	//Run tests
	code := m.Run()

	// You can't defer this because os.Exit doesn't care for defer
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	os.Exit(code)
}

func TestReadFromDbWithDockerDb(t *testing.T) {
	transactions := app.Table{
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
	}
	accounts := app.Table{
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
	}
	dataModel := app.DataModel{
		Tables: map[string]app.Table{
			"transactions": transactions,
			"accounts":     accounts,
		},
	}
	ctx := context.Background()
	payload, err := app.ParseToDataModelObject(ctx, transactions, []byte(`{"object_id": "9283b948-a140-4993-9c41-d5475fda5671"}`))
	if err != nil {
		t.Fatalf("Could not parse payload: %s", err)
	}
	payload_not_in_db, err := app.ParseToDataModelObject(ctx, transactions, []byte(`{"object_id": "6d3a330d-7204-4561-b523-9fa0d518d184"}`))
	if err != nil {
		t.Fatalf("Could not parse payload: %s", err)
	}

	cases := []MockedTestCase{
		{
			name:           "Read boolean field from DB without join",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions"}, FieldName: "isValidated", DataModel: dataModel, Payload: *payload},
			expectedOutput: pgtype.Bool{Bool: true, Valid: true},
		},
		{
			name:           "Read float field from DB without join",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions"}, FieldName: "value", DataModel: dataModel, Payload: *payload},
			expectedOutput: pgtype.Float8{Float64: 10, Valid: true},
		},
		{
			name:           "Read null float field from DB without join",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions"}, FieldName: "value", DataModel: dataModel, Payload: *payload_not_in_db},
			expectedOutput: pgtype.Float8{Float64: 0, Valid: false},
		},
		{
			name:           "Read string field from DB with join",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions", "accounts"}, FieldName: "status", DataModel: dataModel, Payload: *payload},
			expectedOutput: pgtype.Text{String: "VALIDATED", Valid: true},
		},
	}

	repo := PGRepository{db: dbpool, queryBuilder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			val, err := repo.GetDbField(context.Background(), c.readParams)
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
	}
	accounts := app.Table{
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
	}
	dataModel := app.DataModel{
		Tables: map[string]app.Table{
			"transactions": transactions,
			"accounts":     accounts,
		}}

	ctx := context.Background()
	payload, err := app.ParseToDataModelObject(ctx, transactions, []byte(`{"object_id": "9283b948-a140-4993-9c41-d5475fda5671"}`))
	if err != nil {
		t.Fatalf("Could not parse payload: %s", err)
	}
	param := []interface{}{"1234"}
	cases := []MockedTestCase{
		{

			name:           "Direct table read",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions"}, FieldName: "isValidated", DataModel: dataModel, Payload: *payload},
			expectedQuery:  "SELECT transactions.isValidated FROM transactions WHERE transactions.object_id = $1",
			expectedParams: param,
			expectedOutput: pgtype.Bool{Bool: true, Valid: true},
		},
		{
			name:           "Table read with join - bool",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions", "accounts"}, FieldName: "isValidated", DataModel: dataModel, Payload: *payload},
			expectedQuery:  "SELECT accounts.isValidated FROM transactions JOIN accounts ON transactions.account_id = accounts.object_id WHERE transactions.object_id = $1",
			expectedParams: param,
			expectedOutput: pgtype.Bool{Bool: true, Valid: true},
		},
		{
			name:           "Table read with join - string",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions", "accounts"}, FieldName: "status", DataModel: dataModel, Payload: *payload},
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
	}
	accounts := app.Table{
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
	}
	dataModel := app.DataModel{
		Tables: map[string]app.Table{
			"transactions": transactions,
			"accounts":     accounts,
		}}
	ctx := context.Background()
	payload, err := app.ParseToDataModelObject(ctx, transactions, []byte(`{"object_id": "9283b948-a140-4993-9c41-d5475fda5671"}`))
	if err != nil {
		t.Fatalf("Could not parse payload: %s", err)
	}
	param := []interface{}{"1234"}
	cases := []MockedTestCase{
		{

			name:           "Direct table read",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions"}, FieldName: "isValidated", DataModel: dataModel, Payload: *payload},
			expectedQuery:  "SELECT transactions.isValidated FROM transactions WHERE transactions.object_id = $1",
			expectedParams: param,
			expectedOutput: pgtype.Bool{Bool: true, Valid: true},
		},
		{
			name:           "Table read with join - bool",
			readParams:     app.DbFieldReadParams{Path: []string{"transactions", "accounts"}, FieldName: "isValidated", DataModel: dataModel, Payload: *payload},
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
