package pg_repository

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"testing"
	"text/template"
	"time"

	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"golang.org/x/exp/slog"
)

var testDbPool *pgxpool.Pool
var TestRepo *PGRepository

type testParams struct {
	repository *PGRepository
	logger     *slog.Logger
	testIds    map[string]string
}

var globalTestParams testParams

const (
	testDbLifetime = 120 // seconds
	testUser       = "postgres"
	testPassword   = "pwd"
	testHost       = "localhost"
	testDbName     = "marble"
	testPort       = "5432"
)

func stringBuilder(format string, args map[string]string) string {
	var msg bytes.Buffer

	tmpl, err := template.New("").Parse(format)

	if err != nil {
		return format
	}

	tmpl.Execute(&msg, args)
	return msg.String()
}

// embed migrations sql folder
//
//go:embed migrations/*.sql
var embedMigrations embed.FS

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
	databaseURL := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", testUser, testPassword, hostAndPort, testDbName)
	testDbPool, err = pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("Could not connect to database: %s", err)
	}

	if err = pool.Retry(func() error {
		log.Printf("DB connection pool created. Stats: %+v\n", testDbPool.Stat())
		err = testDbPool.Ping(context.Background())
		if err != nil {
			log.Printf("Could not ping database: %s", err)
			return err
		}
		return nil
	}); err != nil {
		log.Fatalf("Could not connect to db: %s", err)
	}

	createTablesSQL := `
	CREATE TABLE transactions_test(
		id uuid DEFAULT uuid_generate_v4(),
		object_id VARCHAR NOT NULL,
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
		valid_from TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  		valid_until TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'INFINITY',
		account_id VARCHAR,
		title VARCHAR,
		value FLOAT,
		isValidated BOOLEAN,
		PRIMARY KEY(id)
	  );

	CREATE INDEX transactions_test_object_id_idx ON transactions_test(object_id, valid_until DESC, valid_from, updated_at);

	CREATE TABLE bank_accounts_test(
		ID UUID DEFAULT uuid_generate_v4(),
		object_id VARCHAR NOT NULL,
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
		valid_from TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  		valid_until TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'INFINITY',
		status VARCHAR,
		PRIMARY KEY(id)
	);

	CREATE INDEX bank_accounts_test_object_id_idx ON bank_accounts_test(object_id, valid_until DESC, valid_from, updated_at);
	`

	insertDataSQL := `
	INSERT INTO bank_accounts_test (
		object_id,
		updated_at,
		status
	  )
	VALUES(
		'{{.BankAccountId}}',
		'2021-01-01T00:00:00Z',
		'VALIDATED'
	  );

	INSERT INTO transactions_test (
		object_id,
		account_id,
		updated_at,
		value,
		isValidated
	  )
	VALUES(
		'{{.TransactionId1}}',
		'{{.BankAccountId}}',
		'2021-01-01T00:00:00Z',
		10,
		true
	  ),(
		'{{.TransactionId2}}',
		'{{.BankAccountId}}',
		'2021-01-01T00:00:00Z',
		NULL,
		false
	  );
	INSERT INTO organizations (
		id,
		name,
  		database_name
	)
	VALUES(
		'{{.OrganizationId}}',
		'Organization 1',
		'marble'
	)
	`

	organizationId, _ := uuid.NewV4()
	bankAccountId, _ := uuid.NewV4()
	transactionId1, _ := uuid.NewV4()
	transactionId2, _ := uuid.NewV4()
	testIds := map[string]string{
		"OrganizationId": organizationId.String(),
		"BankAccountId":  bankAccountId.String(),
		"TransactionId1": transactionId1.String(),
		"TransactionId2": transactionId2.String(),
	}
	insertDataSQL = stringBuilder(insertDataSQL, testIds)
	fmt.Println(insertDataSQL)

	pgConfig := PGConfig{
		ConnectionString: databaseURL,
		MigrationFS:      embedMigrations,
	}
	TestRepo, err = New("DEV", pgConfig)
	logger := slog.New(slog.NewTextHandler(os.Stderr))
	RunMigrations("DEV", pgConfig, logger)
	if _, err := TestRepo.db.Exec(context.Background(), createTablesSQL); err != nil {
		log.Fatalf("Could not create tables: %s", err)
	}
	if _, err := TestRepo.db.Exec(context.Background(), insertDataSQL); err != nil {
		log.Fatalf("Could not insert test data into tables: %s", err)
	}

	globalTestParams = testParams{repository: TestRepo, logger: logger, testIds: testIds}

	//Run tests
	code := m.Run()

	// You can't defer this because os.Exit doesn't care for defer
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	os.Exit(code)
}
