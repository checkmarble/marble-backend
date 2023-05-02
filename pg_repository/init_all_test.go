package pg_repository

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

var testDbPool *pgxpool.Pool
var TestRepo *PGRepository

const (
	testDbLifetime = 120 // seconds
	testUser       = "test_user"
	testPassword   = "pwd"
	testHost       = "localhost"
	testDbName     = "test_db"
	testPort       = "5432"
)

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
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
		valid_from TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  		valid_until TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'INFINITY',
		account_id VARCHAR,
		title VARCHAR,
		value FLOAT,
		isValidated BOOLEAN,
		PRIMARY KEY(id)
	  );

	CREATE INDEX transactions_object_id_idx ON transactions(object_id, valid_until DESC, valid_from, updated_at);

	CREATE TABLE bank_accounts(
		ID UUID DEFAULT uuid_generate_v4(),
		object_id VARCHAR NOT NULL,
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
		valid_from TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  		valid_until TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'INFINITY',
		status VARCHAR,
		PRIMARY KEY(id)
	);

	CREATE INDEX bank_accounts_object_id_idx ON bank_accounts(object_id, valid_until DESC, valid_from, updated_at);

	INSERT INTO bank_accounts (
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

	if _, err := testDbPool.Exec(context.Background(), createTablesSQL); err != nil {
		log.Fatalf("Could not create tables: %s", err)
	}

	TestRepo = &PGRepository{db: testDbPool, queryBuilder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}

	//Run tests
	code := m.Run()

	// You can't defer this because os.Exit doesn't care for defer
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	os.Exit(code)
}
