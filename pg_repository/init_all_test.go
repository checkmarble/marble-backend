package pg_repository

import (
	"context"
	"fmt"
	"log"
	"marble/marble-backend/infra"
	"marble/marble-backend/repositories"
	"os"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"golang.org/x/exp/slog"
)

const uuidRegexp = `^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$`

type testParams struct {
	repository         *PGRepository
	scenarioRepository repositories.ScenarioReadRepository
	logger             *slog.Logger
	testIds            map[string]string
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

// func stringBuilder(format string, args map[string]string) string {
// 	var msg bytes.Buffer

// 	tmpl, err := template.New("").Parse(format)

// 	if err != nil {
// 		return format
// 	}

// 	tmpl.Execute(&msg, args)
// 	return msg.String()
// }

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
	testDbPool, err := pgxpool.New(context.Background(), databaseURL)
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

	pgConfig := PGConfig{ConnectionString: databaseURL}
	logger := slog.New(slog.NewTextHandler(os.Stderr))
	RunMigrations("DEV", pgConfig, logger)

	// Need to declare this after the migrations, to have the correct search path
	anotherPool, err := infra.NewPostgresConnectionPool(pgConfig.GetConnectionString("DEV"))
	if err != nil {
		log.Fatalf("Could not create connection pool: %s", err)
	}
	TestRepo, err := New(anotherPool)

	// insertDataSQL := `
	// INSERT INTO companies (
	// 	object_id,
	// 	updated_at,
	// 	name
	//   )
	// VALUES(
	// 	'{{.CompanyId}}',
	// 	'2021-01-01T00:00:00Z',
	// 	'Test company 1'
	// );

	// INSERT INTO accounts (
	// 	object_id,
	// 	updated_at,
	// 	name,
	// 	currency,
	// 	company_id
	//   )
	// VALUES(
	// 	'{{.BankAccountId}}',
	// 	'2021-01-01T00:00:00Z',
	// 	'SHINE',
	// 	'EUR',
	// 	'{{.CompanyId}}'
	//   );

	// INSERT INTO transactions (
	// 	object_id,
	// 	account_id,
	// 	updated_at,
	// 	amount,
	// 	title
	//   )
	// VALUES(
	// 	'{{.TransactionId}}',
	// 	'{{.BankAccountId}}',
	// 	'2021-01-01T00:00:00Z',
	// 	10,
	// 	'AMAZON'
	//   );

	// INSERT INTO organizations (
	// 	id,
	// 	name,
	// 	database_name
	// )
	// VALUES(
	// 	'{{.OrganizationId}}',
	// 	'Organization 1',
	// 	'marble'
	// )
	// `

	organizationId, _ := uuid.NewV4()
	companyId, _ := uuid.NewV4()
	bankAccountId, _ := uuid.NewV4()
	transactionId, _ := uuid.NewV4()

	testIds := map[string]string{
		"OrganizationId": organizationId.String(),
		"CompanyId":      companyId.String(),
		"BankAccountId":  bankAccountId.String(),
		"TransactionId":  transactionId.String(),
	}

	// insertDataSQL = stringBuilder(insertDataSQL, testIds)
	// if _, err := TestRepo.db.Exec(context.Background(), insertDataSQL); err != nil {
	// 	log.Fatalf("Could not insert test data into tables: %s", err)
	// }

	globalTestParams = testParams{
		repository: TestRepo,
		scenarioRepository: repositories.NewScenarioReadRepositoryPostgresql(
			repositories.NewTransactionFactoryPosgresql(
				nil,
				anotherPool,
			),
		),
		logger:  logger,
		testIds: testIds,
	}

	//Run tests
	code := m.Run()

	// You can't defer this because os.Exit doesn't care for defer
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	os.Exit(code)
}
