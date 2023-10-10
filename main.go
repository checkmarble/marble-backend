package main

import (
	"context"
	"crypto/rsa"
	"errors"
	"flag"
	"log"
	"log/slog"
	"net/http"
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

func runServer(ctx context.Context, appConfig AppConfiguration) {
	jwtSigningKey := utils.GetRequiredStringEnv("AUTHENTICATION_JWT_SIGNING_KEY")
	marbleJwtSigningKey := infra.MustParseSigningKey(jwtSigningKey)

	uc := NewUseCases(ctx, appConfig, marbleJwtSigningKey)

	logger := utils.LoggerFromContext(ctx)

	////////////////////////////////////////////////////////////
	// Seed the database
	////////////////////////////////////////////////////////////

	seedUsecase := uc.NewSeedUseCase()

	marbleAdminEmail, _ := os.LookupEnv("MARBLE_ADMIN_EMAIL")
	if marbleAdminEmail != "" {
		err := seedUsecase.SeedMarbleAdmins(marbleAdminEmail)
		if err != nil {
			panic(err)
		}
	}

	if appConfig.env == "DEV" {
		zorgOrganizationId := "13617a88-56f5-4baa-8d11-ce102f7da907"
		err := seedUsecase.SeedZorgOrganization(zorgOrganizationId)
		if err != nil {
			panic(err)
		}
	}

	router := initRouter(ctx, appConfig)
	server := api.New(router, appConfig.port, uc)

	////////////////////////////////////////////////////////////
	// Start serving the app
	////////////////////////////////////////////////////////////

	// Intercept SIGxxx signals
	notify, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("starting server on port %v\n", appConfig.port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.ErrorContext(ctx, "error serving the app: \n"+err.Error())
		}
		logger.InfoContext(ctx, "server returned")
	}()

	// Block until we receive our signal.
	<-notify.Done()
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.ErrorContext(ctx, "server.Shutdown error", slog.String("error", err.Error()))
	}
}

type AppConfiguration struct {
	env           string
	port          string
	jwtSigningKey string
	gcpProject    string
	pgConfig      utils.PGConfig
	config        models.GlobalConfiguration
}

func main() {
	appConfig := AppConfiguration{
		env:        utils.GetStringEnv("ENV", "DEV"),
		port:       utils.GetRequiredStringEnv("PORT"),
		gcpProject: os.Getenv("GOOGLE_CLOUD_PROJECT"),
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
			FakeGcsRepository:   utils.GetBoolEnv("FAKE_GCS", false),
			GcsIngestionBucket:  utils.GetRequiredStringEnv("GCS_INGESTION_BUCKET"),
		},
	}

	////////////////////////////////////////////////////////////
	// Setup dependencies
	////////////////////////////////////////////////////////////

	logger := utils.NewLogger(appConfig.env)
	appContext := utils.StoreLoggerInContext(context.Background(), logger)

	shouldRunMigrations := flag.Bool("migrations", false, "Run migrations")
	shouldRunServer := flag.Bool("server", false, "Run server")
	shouldRunScheduleScenarios := flag.Bool("scheduler", false, "Run schedule scenarios")
	shouldRunExecuteScheduledScenarios := flag.Bool("scheduled-executer", false, "Run execute scheduled scenarios")
	shouldRunBatchIngestion := flag.Bool("batch-ingestion", false, "Run batch ingestion")
	shouldRunDataIngestion := flag.Bool("data-ingestion", false, "Run data ingestion")
	flag.Parse()
	logger.Info("Flags",
		slog.Bool("shouldRunMigrations", *shouldRunMigrations),
		slog.Bool("shouldRunServer", *shouldRunServer),
		slog.Bool("shouldRunScheduledScenarios", *shouldRunScheduleScenarios),
		slog.Bool("shouldRunBatchIngestion", *shouldRunBatchIngestion),
		slog.Bool("shouldRunDataIngestion", *shouldRunDataIngestion),
	)

	if *shouldRunMigrations {
		err := repositories.RunMigrations(appConfig.env, appConfig.pgConfig, logger)
		if err != nil {
			slog.Error("repositories.RunMigrations failed", slog.String("error", err.Error()))
			os.Exit(1)
			return
		}
	}

	if *shouldRunServer {
		runServer(appContext, appConfig)
	}

	if *shouldRunScheduleScenarios {
		usecases := NewUseCases(appContext, appConfig, nil)
		err := jobs.ScheduleDueScenarios(appContext, usecases)
		if err != nil {
			slog.Error("jobs.ScheduleDueScenarios failed", slog.String("error", err.Error()))
			os.Exit(1)
			return
		}
	}

	if *shouldRunExecuteScheduledScenarios {
		usecases := NewUseCases(appContext, appConfig, nil)
		err := jobs.ExecuteAllScheduledScenarios(appContext, usecases)
		if err != nil {
			slog.Error("jobs.ExecuteAllScheduledScenarios failed", slog.String("error", err.Error()))
			os.Exit(1)
			return
		}
	}

	if *shouldRunBatchIngestion {
		bucketName := appConfig.config.GcsIngestionBucket
		usecases := NewUseCases(appContext, appConfig, nil)
		err := jobs.IngestDataFromStorageCSVs(appContext, usecases, bucketName)
		if err != nil {
			slog.Error("jobs.IngestDataFromStorageCSVs failed", slog.String("error", err.Error()))
			os.Exit(1)
			return
		}
	}

	if *shouldRunDataIngestion {
		usecases := NewUseCases(appContext, appConfig, nil)
		err := jobs.IngestDataFromCsv(appContext, usecases)
		if err != nil {
			slog.Error("jobs.IngestDataFromCsv failed", slog.String("error", err.Error()))
			os.Exit(1)
			return
		}
	}
}

func NewUseCases(ctx context.Context, appConfiguration AppConfiguration, marbleJwtSigningKey *rsa.PrivateKey) usecases.Usecases {
	connectionString := appConfiguration.pgConfig.GetConnectionString(appConfiguration.env)

	marbleConnectionPool, err := infra.NewPostgresConnectionPool(connectionString)
	if err != nil {
		log.Fatal("error creating postgres connection to marble database", err.Error())
	}

	repositories, err := repositories.NewRepositories(
		marbleJwtSigningKey,
		infra.InitializeFirebase(ctx),
		marbleConnectionPool,
		utils.LoggerFromContext(ctx),
	)
	if err != nil {
		slog.Error("repositories.NewRepositories failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	return usecases.Usecases{
		Repositories:  *repositories,
		Configuration: appConfiguration.config,
	}
}
