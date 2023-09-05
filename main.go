package main

import (
	"context"
	"crypto/rsa"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/checkmarble/marble-backend/api"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/jobs"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func runServer(ctx context.Context, usecases usecases.Usecases, port string, isDevEnv bool, projectId string) {

	logger := utils.LoggerFromContext(ctx)

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

	if isDevEnv {
		zorgOrganizationId := "13617a88-56f5-4baa-8d11-ce102f7da907"
		err := seedUsecase.SeedZorgOrganization(zorgOrganizationId)
		if err != nil {
			panic(err)
		}
	}

	api, _ := api.New(ctx, port, usecases, isDevEnv, projectId)

	////////////////////////////////////////////////////////////
	// Start serving the app
	////////////////////////////////////////////////////////////

	// Intercept SIGxxx signals
	notify, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("starting server on port %v\n", port)
		if err := api.ListenAndServe(); err != nil {
			logger.ErrorContext(ctx, "error serving the app: \n"+err.Error())
		}
		logger.InfoContext(ctx, "server returned")
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
	pgConfig utils.PGConfig
	config   models.GlobalConfiguration
}

func main() {

	appConfig := AppConfiguration{
		env:  utils.GetStringEnv("ENV", "DEV"),
		port: utils.GetRequiredStringEnv("PORT"),
		pgConfig: utils.PGConfig{
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
	isDevEnv := appConfig.env == "DEV"

	////////////////////////////////////////////////////////////
	// Setup dependencies
	////////////////////////////////////////////////////////////

	logger := utils.NewLogger(appConfig.env)
	appContext := utils.StoreLoggerInContext(context.Background(), logger)

	shouldRunMigrations := flag.Bool("migrations", false, "Run migrations")
	shouldRunServer := flag.Bool("server", false, "Run server")
	shouldRunScheduledScenarios := flag.Bool("scheduler", false, "Run scheduled scenarios")
	shouldRunBatchIngestion := flag.Bool("batch-ingestion", false, "Run batch ingestion")
	flag.Parse()
	logger.Info("Flags", "shouldRunMigrations", *shouldRunMigrations, "shouldRunServer", *shouldRunServer)

	if *shouldRunMigrations {
		repositories.RunMigrations(appConfig.env, appConfig.pgConfig, logger)
	}

	if *shouldRunServer {
		marbleJwtSigningKey := infra.MustParseSigningKey(utils.GetRequiredStringEnv("AUTHENTICATION_JWT_SIGNING_KEY"))

		usecases := NewUseCases(appContext, appConfig, &marbleJwtSigningKey)
		runServer(appContext, usecases, appConfig.port, isDevEnv, os.Getenv("GOOGLE_CLOUD_PROJECT"))
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

	repositories, err := repositories.NewRepositories(
		appConfiguration.config,
		marbleJwtSigningKey,
		infra.IntializeFirebase(ctx),
		marbleConnectionPool,
		utils.LoggerFromContext(ctx),
	)
	if err != nil {
		panic(err)
	}

	return usecases.Usecases{
		Repositories:  *repositories,
		Configuration: appConfiguration.config,
	}

}
