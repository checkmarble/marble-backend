package main

import (
	"context"
	"crypto/rsa"
	"flag"
	"fmt"
	"log"
	"marble/marble-backend/api"
	"marble/marble-backend/infra"
	"marble/marble-backend/jobs"
	"marble/marble-backend/models"
	"marble/marble-backend/pg_repository"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases"
	"marble/marble-backend/utils"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/exp/slog"
)

func runServer(ctx context.Context, usecases usecases.Usecases, port string, devEnv bool) {

	logger := utils.LoggerFromContext(ctx)

	corsAllowLocalhost := devEnv

	////////////////////////////////////////////////////////////
	// Seed the database
	////////////////////////////////////////////////////////////

	seedUsecase := usecases.NewSeedUseCase()

	marbleAdminEmail, _ := os.LookupEnv("MARBLE_ADMIN_EMAIL")
	if marbleAdminEmail != "" {
		err := seedUsecase.SeedMarbleAdmins(marbleAdminEmail)
		if err != nil {
			panic(err)
		}
	}

	if devEnv {
		zorgOrganizationId := "13617a88-56f5-4baa-8d11-ce102f7da907"
		err := seedUsecase.SeedZorgOrganization(zorgOrganizationId)
		if err != nil {
			panic(err)
		}
	}

	api, _ := api.New(ctx, port, usecases, corsAllowLocalhost)

	////////////////////////////////////////////////////////////
	// Start serving the app
	////////////////////////////////////////////////////////////

	// Intercept SIGxxx signals
	notify, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("starting server on port %v\n", port)
		if err := api.ListenAndServe(); err != nil {
			logger.ErrorCtx(ctx, "error serving the app: \n"+err.Error())
		}
		logger.InfoCtx(ctx, "server returned")
	}()

	// Block until we receive our signal.
	<-notify.Done()
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	api.Shutdown(shutdownCtx)
}

type AppConfiguration struct {
	env      string
	port     string
	pgConfig pg_repository.PGConfig
	config   models.GlobalConfiguration
}

func main() {

	appConfig := AppConfiguration{
		env:  utils.GetStringEnv("ENV", "DEV"),
		port: utils.GetRequiredStringEnv("PORT"),
		pgConfig: pg_repository.PGConfig{
			Hostname: utils.GetRequiredStringEnv("PG_HOSTNAME"),
			Port:     utils.GetStringEnv("PG_PORT", "5432"),
			User:     utils.GetRequiredStringEnv("PG_USER"),
			Password: utils.GetRequiredStringEnv("PG_PASSWORD"),
			Database: "marble",
		},
		config: models.GlobalConfiguration{
			TokenLifetimeMinute: utils.GetIntEnv("TOKEN_LIFETIME_MINUTE", 60*2),
			FakeAwsS3Repository: utils.GetBoolEnv("FAKE_AWS_S3", false),
		},
	}

	////////////////////////////////////////////////////////////
	// Setup dependencies
	////////////////////////////////////////////////////////////

	var logger *slog.Logger

	devEnv := appConfig.env == "DEV"
	if devEnv {
		textHandler := slog.HandlerOptions{ReplaceAttr: utils.LoggerAttributeReplacer}.NewTextHandler(os.Stderr)
		logger = slog.New(textHandler)
	} else {
		jsonHandler := slog.HandlerOptions{ReplaceAttr: utils.LoggerAttributeReplacer}.NewJSONHandler(os.Stderr)
		logger = slog.New(jsonHandler)
	}

	shouldRunMigrations := flag.Bool("migrations", false, "Run migrations")
	shouldRunServer := flag.Bool("server", false, "Run server")
	shouldRunScheduledScenarios := flag.Bool("scheduler", false, "Run scheduled scenarios")
	shouldRunBatchIngestion := flag.Bool("batch-ingestion", false, "Run batch ingestion")
	flag.Parse()
	logger.DebugCtx(context.Background(), "shouldRunMigrations", *shouldRunMigrations, "shouldRunServer", *shouldRunServer)

	if *shouldRunMigrations {
		pg_repository.RunMigrations(appConfig.env, appConfig.pgConfig, logger)
	}

	appContext := utils.StoreLoggerInContext(context.Background(), logger)

	if *shouldRunServer {

		marbleJwtSigningKey := infra.MustParseSigningKey(utils.GetRequiredStringEnv("AUTHENTICATION_JWT_SIGNING_KEY"))

		usecases := NewUseCases(appContext, appConfig, &marbleJwtSigningKey)
		runServer(appContext, usecases, appConfig.port, devEnv)
	}

	if *shouldRunScheduledScenarios {
		usecases := NewUseCases(appContext, appConfig, nil)
		jobs.ExecuteAllScheduledScenarios(appContext, usecases)
	}

	if *shouldRunBatchIngestion {
		bucketName := utils.GetRequiredStringEnv("GCS_INGESTION_BUCKET")
		usecases := NewUseCases(appContext, appConfig, nil)
		jobs.IngestDataFromStorageCSVs(appContext, usecases, bucketName)
	}
}

func NewUseCases(ctx context.Context, appConfiguration AppConfiguration, marbleJwtSigningKey *rsa.PrivateKey) usecases.Usecases {
	connectionString := appConfiguration.pgConfig.GetConnectionString(appConfiguration.env)

	marbleConnectionPool, err := infra.NewPostgresConnectionPool(connectionString)
	if err != nil {
		log.Fatal("error creating postgres connection to marble database", err.Error())
	}

	pgRepository, err := pg_repository.New(marbleConnectionPool)
	if err != nil {
		panic(fmt.Errorf("error creating pg repository %w", err))
	}

	repositories, err := repositories.NewRepositories(
		appConfiguration.config,
		marbleJwtSigningKey,
		infra.IntializeFirebase(ctx),
		marbleConnectionPool,
		utils.LoggerFromContext(ctx),
		pgRepository,
		pgRepository,
		pgRepository,
	)
	if err != nil {
		panic(err)
	}

	return usecases.Usecases{
		Repositories:  *repositories,
		Configuration: appConfiguration.config,
	}

}
