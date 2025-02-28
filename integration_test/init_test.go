package integration

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/checkmarble/marble-backend/api"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/jobs"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/firebase"
	"github.com/checkmarble/marble-backend/repositories/postgres"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/token"
	"github.com/checkmarble/marble-backend/utils"
)

const (
	testDbLifetime   = 120 // seconds
	testUser         = "postgres"
	testPassword     = "pwd"
	testDbName       = "marble"
	marbleAdminEmail = "test@admin.com"
)

var (
	testUsecases   usecases.Usecases
	tokenGenerator *token.Generator
	riverClient    *river.Client[pgx.Tx]

	testServer *httptest.Server
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

	err = resource.Expire(testDbLifetime) // Tell docker to hard kill the container in testDbLifetime seconds
	if err != nil {
		log.Fatalf("Could not set container lifetime: %s", err)
	}

	pool.MaxWait = testDbLifetime * time.Second

	hostAndPort := resource.GetHostPort("5432/tcp") // docker container will bind to another port than 5432 if already taken
	connectionString := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", testUser, testPassword, hostAndPort, testDbName)
	testDbPool, err := pgxpool.New(context.Background(), connectionString)
	if err != nil {
		log.Fatalf("Could not connect to database: %s", err)
	}
	log.Printf("DB connection pool created.")

	if err = pool.Retry(func() error {
		err = testDbPool.Ping(ctx)
		if err != nil {
			log.Printf("Could not ping database: %s", err)
			return err
		}
		return nil
	}); err != nil {
		log.Fatalf("Could not connect to db: %s", err)
	}

	pgConfig := infra.PgConfig{ConnectionString: connectionString}
	migrater := repositories.NewMigrater(pgConfig)
	logger := utils.NewLogger("text")
	ctx = utils.StoreLoggerInContext(ctx, logger)

	err = migrater.Run(ctx)
	if err != nil {
		log.Fatalf("Could not run migrations: %s", err)
	}

	// Need to declare this after the migrations, to have the correct search path
	dbPool, err := infra.NewPostgresConnectionPool(ctx, pgConfig.GetConnectionString(), nil, pgConfig.MaxPoolConnections)
	if err != nil {
		log.Fatalf("Could not create connection pool: %s", err)
	}

	privateKey := infra.ReadParseOrGenerateSigningKey(ctx, "", "")

	workers := river.NewWorkers()
	// AddWorker panics if the worker is already registered or invalid

	riverClient, err = river.NewClient(riverpgxv5.New(dbPool), &river.Config{
		Workers: workers,
		// The org specific queues are added later, dynamically, in the test (they rely on the org id)
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {
				MaxWorkers: 3,
			},
		},
	})
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		log.Fatalf("Could not create river client: %s", err)
	}

	// actually using the convoy repository to send webhooks will fail (because we don't have an instance set up),
	// but it is not blocking (an error will be logged but the test will pass). We sill need to pass the provider
	// or else the repository will panic.
	repos := repositories.NewRepositories(dbPool,
		"",
		repositories.WithConvoyClientProvider(
			infra.InitializeConvoyRessources(infra.ConvoyConfiguration{}), 0),
		repositories.WithRiverClient(riverClient),
	)

	testUsecases = usecases.NewUsecases(repos,
		usecases.WithLicense(models.NewFullLicense()),
		usecases.WithIngestionBucketUrl("file://./tempFiles?create_dir=true"),
		usecases.WithCaseManagerBucketUrl("file://./tempFiles?create_dir=true"),
	)

	adminUc := jobs.GenerateUsecaseWithCredForMarbleAdmin(ctx, testUsecases)
	river.AddWorker(workers, adminUc.NewAsyncDecisionWorker())
	river.AddWorker(workers, adminUc.NewNewAsyncScheduledExecWorker())
	river.AddWorker(workers, adminUc.NewIndexCreationWorker())
	river.AddWorker(workers, adminUc.NewIndexCreationStatusWorker())

	if err := riverClient.Start(ctx); err != nil {
		log.Fatalln("Could not start river client:", err)
	}

	apiConfig := api.Configuration{
		Env:                 "development",
		AppName:             "marble-backend",
		MarbleAppUrl:        "http://localhost:3000",
		RequestLoggingLevel: "all",
		TokenLifetimeMinute: 60,
		SegmentWriteKey:     "",
		BatchTimeout:        55 * time.Second,
		DecisionTimeout:     10 * time.Second,
		DefaultTimeout:      5 * time.Second,
	}
	tokenVerifier := infra.NewMockedFirebaseTokenVerifier()
	firebaseClient := firebase.New(tokenVerifier)
	deps := api.InitDependencies(ctx, apiConfig, dbPool, privateKey, tokenVerifier)

	telemetryRessources, _ := infra.InitTelemetry(infra.TelemetryConfiguration{Enabled: false}, "")
	router := api.InitRouterMiddlewares(ctx, apiConfig, deps.SegmentClient, telemetryRessources)
	server := api.NewServer(router, apiConfig, testUsecases,
		deps.Authentication, deps.TokenHandler, logger, api.WithLocalTest(true))

	jwtRepository := repositories.NewJWTRepository(privateKey)
	database := postgres.New(dbPool)
	if err != nil {
		panic(err)
	}
	tokenGenerator = token.NewGenerator(database, jwtRepository, firebaseClient, 10)

	// we need to create a first marble admin user, otherwise we can't use the API (chicken and egg)
	seedUsecase := testUsecases.NewSeedUseCase()
	if err := seedUsecase.SeedMarbleAdmins(ctx, marbleAdminEmail); err != nil {
		logger.ErrorContext(ctx, "Error seeding marble admin", "error", err)
		panic(err)
	}

	testServer = httptest.NewServer(server.Handler)
	defer testServer.Close()

	logger.InfoContext(ctx, "started server", slog.String("url", testServer.URL))

	// Run tests
	code := m.Run()

	_ = server.Shutdown(ctx)

	_ = riverClient.Stop(ctx)

	// You can't defer this because os.Exit doesn't care for defer
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	os.Exit(code)
}
