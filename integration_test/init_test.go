package integration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/postgres"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/token"
	"github.com/checkmarble/marble-backend/utils"
)

const (
	testDbLifetime = 120 // seconds
	testUser       = "postgres"
	testPassword   = "pwd"
	testDbName     = "marble"
)

var (
	testUsecases   usecases.Usecases
	tokenGenerator *token.Generator
)

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
		Repository: "europe-west1-docker.pkg.dev/marble-infra/marble/postgresql-db", // custom image of postgres15 with pg_cron extension added
		Tag:        "latest",
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

	pgConfig := utils.PGConfig{ConnectionString: databaseURL}
	migrater := repositories.NewMigrater(pgConfig)
	err = migrater.Run(ctx)
	if err != nil {
		log.Fatalf("Could not run migrations: %s", err)
	}

	// Need to declare this after the migrations, to have the correct search path
	dbPool, err := infra.NewPostgresConnectionPool(pgConfig.GetConnectionString())
	if err != nil {
		log.Fatalf("Could not create connection pool: %s", err)
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		log.Fatalf("Could not create private key: %s", err)
	}
	repos, err := repositories.NewRepositories(
		privateKey,
		nil,
		dbPool,
		nil,
	)
	if err != nil {
		panic(err)
	}
	jwtRepository := repositories.NewJWTRepository(privateKey)
	database, err := postgres.New(postgres.Configuration{
		Host:                strings.Split(hostAndPort, ":")[0],
		Port:                strings.Split(hostAndPort, ":")[1],
		User:                testUser,
		Password:            testPassword,
		Database:            testDbName,
		DbConnectWithSocket: false,
	})
	if err != nil {
		panic(err)
	}
	tokenGenerator = token.NewGenerator(database, jwtRepository, nil, 10)

	testUsecases = usecases.Usecases{
		Repositories: *repos,
	}

	// Run tests
	code := m.Run()

	// You can't defer this because os.Exit doesn't care for defer
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	os.Exit(code)
}
