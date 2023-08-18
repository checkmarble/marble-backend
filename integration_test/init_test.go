package integration_test

import (
	"context"
	"fmt"
	"log"
	"marble/marble-backend/infra"
	"marble/marble-backend/models"
	"marble/marble-backend/pg_repository"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases"
	"marble/marble-backend/utils"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"golang.org/x/exp/slog"
)

const (
	testDbLifetime = 120 // seconds
	testUser       = "postgres"
	testPassword   = "pwd"
	testHost       = "localhost"
	testDbName     = "marble"
	testPort       = "5432"
)

var testUsecases usecases.Usecases

func TestMain(m *testing.M) {
	ctx := context.Background()
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
		err = testDbPool.Ping(ctx)
		if err != nil {
			log.Printf("Could not ping database: %s", err)
			return err
		}
		return nil
	}); err != nil {
		log.Fatalf("Could not connect to db: %s", err)
	}

	pgConfig := pg_repository.PGConfig{ConnectionString: databaseURL}
	logger := slog.New(slog.NewTextHandler(os.Stderr))
	pg_repository.RunMigrations("DEV", pgConfig, logger)

	// Need to declare this after the migrations, to have the correct search path
	dbPool, err := infra.NewPostgresConnectionPool(pgConfig.GetConnectionString("DEV"))
	if err != nil {
		log.Fatalf("Could not create connection pool: %s", err)
	}

	pgRepository, err := pg_repository.New(dbPool)
	if err != nil {
		panic(fmt.Errorf("error creating pg repository %w", err))
	}

	appContext := utils.StoreLoggerInContext(ctx, logger)

	repositories, err := repositories.NewRepositories(
		models.GlobalConfiguration{},
		nil,
		nil,
		dbPool,
		utils.LoggerFromContext(appContext),
		pgRepository,
	)
	if err != nil {
		panic(err)
	}

	testUsecases = usecases.Usecases{
		Repositories: *repositories,
	}

	//Run tests
	code := m.Run()

	// You can't defer this because os.Exit doesn't care for defer
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	os.Exit(code)
}
