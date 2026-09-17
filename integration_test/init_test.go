package integration

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/api"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/jobs"
	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/clock"
	"github.com/checkmarble/marble-backend/repositories/idp"
	"github.com/checkmarble/marble-backend/repositories/postgres"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/auth"
	"github.com/checkmarble/marble-backend/usecases/worker_jobs"
	"github.com/checkmarble/marble-backend/utils"
)

const (
	testDbLifetime   = 180     // seconds
	testUser         = "admin" // Nb: not using the default "postgres" on purpose, to verify the migrations run even with a different user
	testPassword     = "pwd"
	testDbName       = "marble_db" // Nb: not using the default "marble" on purpose, to verify the migrations run even with a different db name
	marbleAdminEmail = "test@admin.com"
)

var (
	testUsecases   usecases.Usecases
	apiKeyVerifier auth.Verifier
	tokenGenerator auth.TokenGenerator
	riverClient    *river.Client[pgx.Tx]
	pgPool         *pgxpool.Pool

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
		Repository: "postgis/postgis",
		Tag:        "18-3.6-alpine",
		Platform:   "linux/amd64",
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

	var purgeOnce sync.Once
	var purgeErr error
	purgeResource := func() error {
		purgeOnce.Do(func() {
			purgeErr = pool.Purge(resource)
		})
		return purgeErr
	}

	var dbDeadline *time.Timer
	cleanupAndFatal := func(format string, args ...any) {
		if dbDeadline != nil {
			dbDeadline.Stop()
		}
		if err := purgeResource(); err != nil {
			log.Printf("failed to purge PostgreSQL container during setup failure: %v", err)
		}
		log.Fatalf(format, args...)
	}

	dbDeadline = time.AfterFunc(testDbLifetime*time.Second, func() {
		log.Printf("integration-test database lifetime reached after %s; removing PostgreSQL container %s",
			testDbLifetime*time.Second, resource.Container.ID)

		if err := purgeResource(); err != nil {
			log.Printf("failed to remove timed-out PostgreSQL container: %v", err)
		}
	})

	pool.MaxWait = testDbLifetime * time.Second

	hostAndPort := resource.GetHostPort("5432/tcp") // docker container will bind to another port than 5432 if already taken

	if os.Getenv("_INTERNAL_TEST_USE_CONTAINER_IP") == "1" {
		hostAndPort = fmt.Sprintf("%s:5432", resource.Container.NetworkSettings.IPAddress)
	}

	connectionString := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", testUser, testPassword, hostAndPort, testDbName)
	testDbPool, err := pgxpool.New(context.Background(), connectionString)
	if err != nil {
		cleanupAndFatal("Could not connect to database: %s", err)
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
		cleanupAndFatal("Could not connect to db: %s", err)
	}

	pgConfig := infra.PgConfig{ConnectionString: connectionString}
	migrater := repositories.NewMigrater(pgConfig)
	logger := utils.NewLogger("text")
	ctx = utils.StoreLoggerInContext(ctx, logger)

	err = migrater.Run(ctx, nil)
	if err != nil {
		cleanupAndFatal("Could not run migrations: %s", err)
	}

	// Need to declare this after the migrations, to have the correct search path
	dbPool, err := infra.NewPostgresConnectionPool(ctx, "marble-test",
		pgConfig.GetConnectionString(), nil, pgConfig.MaxPoolConnections, "")
	if err != nil {
		cleanupAndFatal("Could not create connection pool: %s", err)
	}

	pgPool = dbPool

	privateKey := infra.ReadParseOrGenerateSigningKey(ctx, "", "")

	workers := river.NewWorkers()
	// AddWorker panics if the worker is already registered or invalid
	// Register workers so that job enqueueing doesn't fail validation
	river.AddWorker(workers, usecases.NewCsvIngestionWorker(nil))
	river.AddWorker(workers, worker_jobs.NewScheduledExecutionWorker(nil))

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
		cleanupAndFatal("Could not create river client: %s", err)
	}

	mredis := miniredis.NewMiniRedis()
	_ = mredis.Start()
	redisClient, _ := repositories.NewRedisClient(infra.RedisConfig{Address: mredis.Addr()})

	repos := repositories.NewRepositories(
		dbPool,
		infra.GcpConfig{},
		repositories.WithRiverClient(riverClient),
		repositories.WithRedisClient(redisClient),
	)

	firebaseAdminClient := &mocks.FirebaseAdminClient{}
	firebaseAdminClient.On("CreateUser", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	testUsecases = usecases.NewUsecases(
		repos,
		usecases.WithAppName("marble-test"),
		usecases.WithLicense(models.NewFullLicense()),
		usecases.WithIngestionBucketUrl("file:///tmp/tempFiles?create_dir=true"),
		usecases.WithCaseManagerBucketUrl("file:///tmp/tempFiles?create_dir=true"),
		usecases.WithOffloadingBucketUrl("file:///tmp/tempFiles?create_dir=true"),
		usecases.WithFirebaseAdmin(auth.TokenProviderFirebase, firebaseAdminClient),
	)

	adminUc := jobs.GenerateUsecaseWithCredForMarbleAdmin(ctx, testUsecases)
	river.AddWorker(workers, adminUc.NewAsyncDecisionWorker())
	river.AddWorker(workers, adminUc.NewNewAsyncScheduledExecWorker())
	river.AddWorker(workers, adminUc.NewBatchExecutionCoordinatorWorker())
	river.AddWorker(workers, adminUc.NewIndexCreationWorker())
	river.AddWorker(workers, adminUc.NewIndexCreationStatusWorker())
	river.AddWorker(workers, adminUc.NewCaseReviewWorker(10*time.Second))
	river.AddWorker(workers, adminUc.NewRuleDescriptionWorker(10*time.Second))
	river.AddWorker(workers, adminUc.NewDecisionWorkflowsWorker())
	river.AddWorker(workers, adminUc.NewContinuousScreeningDoScreeningWorker())
	river.AddWorker(workers, adminUc.NewWebhookDispatchWorker())

	if err := riverClient.Start(ctx); err != nil {
		cleanupAndFatal("Could not start river client: %s", err)
	}

	apiConfig := api.Configuration{
		Env:                 "development",
		AppName:             "marble-backend",
		MarbleAppUrl:        "http://localhost:3000",
		RequestLoggingLevel: "all",
		TokenProvider:       auth.TokenProviderFirebase,
		TokenLifetimeMinute: 60,
		DisableSegment:      true,
		SegmentWriteKey:     "",
		BatchTimeout:        55 * time.Second,
		DecisionTimeout:     10 * time.Second,
		DefaultTimeout:      5 * time.Second,
		FirebaseConfig: api.FirebaseConfig{
			ProjectId: "project",
		},
	}

	tokenVerifier := infra.NewMockedFirebaseTokenVerifier()
	firebaseClient := idp.NewFirebaseClient("project", tokenVerifier)

	deps, _ := api.InitDependencies(ctx, apiConfig, dbPool, privateKey, tokenVerifier)

	telemetryRessources, _ := infra.InitTelemetry(infra.TelemetryConfiguration{Enabled: false}, "")
	router := api.InitRouterMiddlewares(ctx, apiConfig, apiConfig.DisableSegment,
		deps.SegmentClient, telemetryRessources)
	server := api.NewServer(router, apiConfig, testUsecases,
		deps.Authentication, deps.TokenHandler, logger, api.WithLocalTest(true))

	jwtRepository := repositories.NewJWTRepository(infra.MockFirebaseIssuer, privateKey)
	database := postgres.New(dbPool)
	if err != nil {
		cleanupAndFatal("Could not initialize API dependencies: %s", err)
	}

	apiKeyVerifier = auth.NewVerifier(auth.TokenProviderFirebase, firebaseClient, database, nil)
	tokenGenerator = auth.NewGenerator(database, jwtRepository, time.Minute, clock.New())

	// we need to create a first marble admin user, otherwise we can't use the API (chicken and egg)
	seedUsecase := testUsecases.NewSeedUseCase()
	if err := seedUsecase.SeedMarbleAdmins(ctx, marbleAdminEmail); err != nil {
		logger.ErrorContext(ctx, "Error seeding marble admin", "error", err)
		cleanupAndFatal("Could not seed marble admin: %s", err)
	}

	testServer = httptest.NewServer(server.Handler)
	defer testServer.Close()

	logger.InfoContext(ctx, "started server", slog.String("url", testServer.URL))

	// Run tests
	code := m.Run()

	_ = server.Shutdown(ctx)

	_ = riverClient.Stop(ctx)

	dbDeadline.Stop()

	// You can't defer this because os.Exit doesn't care for defer
	if err := purgeResource(); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	os.Exit(code)
}
