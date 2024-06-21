package main

import (
	"context"
	"crypto/rsa"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/analytics-go/v3"

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

type dependencies struct {
	Authentication      *api.Authentication
	TokenHandler        *api.TokenHandler
	SegmentClient       analytics.Client
	TelemetryRessources infra.TelemetryRessources
}

func initDependencies(ctx context.Context, conf AppConfiguration, dbPool *pgxpool.Pool, signingKey *rsa.PrivateKey) dependencies {
	database := postgres.New(dbPool)

	auth := infra.InitializeFirebase(ctx)
	firebaseClient := firebase.New(auth)
	jwtRepository := repositories.NewJWTRepository(signingKey)
	tokenValidator := token.NewValidator(database, jwtRepository)
	tokenGenerator := token.NewGenerator(database, jwtRepository, firebaseClient, conf.config.TokenLifetimeMinute)
	segmentClient := analytics.New(conf.config.SegmentWriteKey)

	return dependencies{
		Authentication:      api.NewAuthentication(tokenValidator),
		SegmentClient:       segmentClient,
		TelemetryRessources: infra.NoopTelemetry(),
		TokenHandler:        api.NewTokenHandler(tokenGenerator),
	}
}

func runServer(ctx context.Context, conf AppConfiguration, tracingConfig infra.TelemetryConfiguration) error {
	marbleConnectionPool, err := infra.NewPostgresConnectionPool(ctx, conf.pgConfig.GetConnectionString())
	if err != nil {
		return err
	}

	uc := NewUseCases(ctx, conf, marbleConnectionPool)

	logger := utils.LoggerFromContext(ctx)

	////////////////////////////////////////////////////////////
	// Seed the database
	////////////////////////////////////////////////////////////
	seedUsecase := uc.NewSeedUseCase()
	marbleAdminEmail := conf.seedOrgConfig.CreateGlobalAdminEmail
	if marbleAdminEmail != "" {
		if err := seedUsecase.SeedMarbleAdmins(ctx, marbleAdminEmail); err != nil {
			return err
		}
	}
	if conf.seedOrgConfig.CreateOrgName != "" {
		if err := seedUsecase.CreateOrgAndUser(ctx, models.InitOrgInput{
			OrgName:    conf.seedOrgConfig.CreateOrgName,
			AdminEmail: conf.seedOrgConfig.CreateOrgAdminEmail,
		}); err != nil {
			return err
		}
	}

	marbleJwtSigningKey := infra.ParseOrGenerateSigningKey(ctx, conf.config.JwtSigningKey)
	deps := initDependencies(ctx, conf, marbleConnectionPool, marbleJwtSigningKey)

	deps.TelemetryRessources, err = infra.InitTelemetry(tracingConfig)
	if err != nil {
		return err
	}

	router := initRouter(ctx, conf, deps)
	server := api.New(router, conf.port, conf.config, uc, deps.Authentication, deps.TokenHandler)

	notify, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.InfoContext(ctx, "starting server", slog.String("port", conf.port))
		err := server.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			logger.ErrorContext(ctx, "error serving the app: \n"+err.Error())
		}
		logger.InfoContext(ctx, "server returned")
	}()

	<-notify.Done()
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	deps.SegmentClient.Close()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.ErrorContext(ctx, "server.Shutdown error", slog.String("error", err.Error()))
	}

	return nil
}

type AppConfiguration struct {
	appName             string
	env                 string
	port                string
	gcpProject          string
	enableGcpTracing    bool
	requestLoggingLevel string
	loggingFormat       string
	pgConfig            infra.PGConfig
	config              models.GlobalConfiguration
	sentryDsn           string
	metabase            models.MetabaseConfiguration
	seedOrgConfig       models.SeedOrgConfiguration
}

func main() {
	config := AppConfiguration{
		appName:             "marble-backend",
		env:                 utils.GetEnv("ENV", "development"),
		port:                utils.GetRequiredEnv[string]("PORT"),
		gcpProject:          os.Getenv("GOOGLE_CLOUD_PROJECT"),
		enableGcpTracing:    utils.GetEnv("ENABLE_GCP_TRACING", false),
		requestLoggingLevel: utils.GetEnv("REQUEST_LOGGING_LEVEL", "all"),
		loggingFormat:       utils.GetEnv("LOGGING_FORMAT", "text"),
		pgConfig: infra.PGConfig{
			Database:            "marble",
			DbConnectWithSocket: utils.GetEnv("PG_CONNECT_WITH_SOCKET", false),
			Hostname:            utils.GetRequiredEnv[string]("PG_HOSTNAME"),
			Password:            utils.GetRequiredEnv[string]("PG_PASSWORD"),
			Port:                utils.GetEnv("PG_PORT", "5432"),
			User:                utils.GetRequiredEnv[string]("PG_USER"),
		},
		config: models.GlobalConfiguration{
			TokenLifetimeMinute:              utils.GetEnv("TOKEN_LIFETIME_MINUTE", 60*2),
			FakeAwsS3Repository:              utils.GetEnv("FAKE_AWS_S3", false),
			FakeGcsRepository:                utils.GetEnv("FAKE_GCS", false),
			GcsIngestionBucket:               utils.GetRequiredEnv[string]("GCS_INGESTION_BUCKET"),
			GcsCaseManagerBucket:             utils.GetRequiredEnv[string]("GCS_CASE_MANAGER_BUCKET"),
			GcsTransferCheckEnrichmentBucket: utils.GetEnv("GCS_TRANSFER_CHECK_ENRICHMENT_BUCKET", ""), // required for transfercheck
			MarbleAppHost:                    utils.GetEnv("MARBLE_APP_HOST", ""),
			MarbleBackofficeHost:             utils.GetEnv("MARBLE_BACKOFFICE_HOST", ""),
			SegmentWriteKey:                  utils.GetRequiredEnv[string]("SEGMENT_WRITE_KEY"),
			JwtSigningKey:                    utils.GetEnv("AUTHENTICATION_JWT_SIGNING_KEY", ""),
		},
		sentryDsn: utils.GetEnv("SENTRY_DSN", ""),
		metabase: models.MetabaseConfiguration{
			SiteUrl:             utils.GetRequiredEnv[string]("METABASE_SITE_URL"),
			JwtSigningKey:       []byte(utils.GetRequiredEnv[string]("METABASE_JWT_SIGNING_KEY")),
			TokenLifetimeMinute: utils.GetEnv("METABASE_TOKEN_LIFETIME_MINUTE", 10),
			Resources: map[models.EmbeddingType]int{
				models.GlobalDashboard: utils.GetRequiredEnv[int]("METABASE_GLOBAL_DASHBOARD_ID"),
			},
		},
		seedOrgConfig: models.SeedOrgConfiguration{
			CreateGlobalAdminEmail: utils.GetEnv("CREATE_GLOBAL_ADMIN_EMAIL", ""),
			CreateOrgName:          utils.GetEnv("CREATE_ORG_NAME", ""),
			CreateOrgAdminEmail:    utils.GetEnv("CREATE_ORG_ADMIN_EMAIL", ""),
		},
	}

	////////////////////////////////////////////////////////////
	// Setup dependencies
	////////////////////////////////////////////////////////////

	logger := utils.NewLogger(config.loggingFormat)
	ctx := utils.StoreLoggerInContext(context.Background(), logger)

	shouldRunMigrations := flag.Bool("migrations", false, "Run migrations")
	shouldRunServer := flag.Bool("server", false, "Run server")
	shouldRunScheduleScenarios := flag.Bool("scheduler", false, "Run schedule scenarios")
	shouldRunExecuteScheduledScenarios := flag.Bool("scheduled-executer", false, "Run execute scheduled scenarios")
	shouldRunDataIngestion := flag.Bool("data-ingestion", false, "Run data ingestion")
	shouldRunScheduler := flag.Bool("cron-scheduler", false, "Run scheduler for cron jobs")
	flag.Parse()
	logger.InfoContext(ctx, "Flags",
		slog.Bool("shouldRunMigrations", *shouldRunMigrations),
		slog.Bool("shouldRunServer", *shouldRunServer),
		slog.Bool("shouldRunScheduledScenarios", *shouldRunScheduleScenarios),
		slog.Bool("shouldRunDataIngestion", *shouldRunDataIngestion),
		slog.Bool("shouldRunScheduler", *shouldRunScheduler),
	)

	infra.SetupSentry(config.sentryDsn, config.env)
	defer sentry.Flush(3 * time.Second)

	tracingConfig := infra.TelemetryConfiguration{
		ApplicationName: config.appName,
		Enabled:         config.enableGcpTracing,
		ProjectID:       config.gcpProject,
	}

	if *shouldRunMigrations {
		migrater := repositories.NewMigrater(config.pgConfig)
		if err := migrater.Run(ctx); err != nil {
			logger.ErrorContext(ctx, fmt.Sprintf(
				"error while running migrations: %+v", err))
			return
		}
	}

	if *shouldRunServer {
		err := runServer(ctx, config, tracingConfig)
		if err != nil {
			logger.ErrorContext(ctx, fmt.Sprintf(
				"error while running server: %+v", err))
			return
		}
	}

	if *shouldRunScheduleScenarios {
		pool, err := infra.NewPostgresConnectionPool(ctx,
			config.pgConfig.GetConnectionString())
		if err != nil {
			logger.ErrorContext(ctx, "failed to create marbleConnectionPool", slog.String("error", err.Error()))
		}
		usecases := NewUseCases(ctx, config, pool)
		err = jobs.ScheduleDueScenarios(ctx, usecases, tracingConfig)
		if err != nil {
			logger.ErrorContext(ctx, "jobs.ScheduleDueScenarios failed", slog.String("error", err.Error()))
			return
		}
	}

	if *shouldRunExecuteScheduledScenarios {
		pool, err := infra.NewPostgresConnectionPool(ctx,
			config.pgConfig.GetConnectionString())
		if err != nil {
			logger.ErrorContext(ctx, "failed to create marbleConnectionPool", slog.String("error", err.Error()))
		}
		usecases := NewUseCases(ctx, config, pool)
		err = jobs.ExecuteAllScheduledScenarios(ctx, usecases, tracingConfig)
		if err != nil {
			logger.ErrorContext(ctx, "jobs.ExecuteAllScheduledScenarios failed", slog.String("error", err.Error()))
			return
		}
	}

	if *shouldRunDataIngestion {
		pool, err := infra.NewPostgresConnectionPool(ctx,
			config.pgConfig.GetConnectionString())
		if err != nil {
			logger.ErrorContext(ctx, "failed to create marbleConnectionPool", slog.String("error", err.Error()))
		}
		usecases := NewUseCases(ctx, config, pool)
		err = jobs.IngestDataFromCsv(ctx, usecases, tracingConfig)
		if err != nil {
			logger.ErrorContext(ctx, "jobs.IngestDataFromCsv failed", slog.String("error", err.Error()))
			return
		}
	}

	if *shouldRunScheduler {
		pool, err := infra.NewPostgresConnectionPool(ctx,
			config.pgConfig.GetConnectionString())
		if err != nil {
			logger.ErrorContext(ctx, "failed to create marbleConnectionPool", slog.String("error", err.Error()))
		}
		jobs.RunScheduler(ctx, NewUseCases(ctx, config, pool), tracingConfig)
	}
}

func NewUseCases(ctx context.Context, appConfiguration AppConfiguration, pool *pgxpool.Pool) usecases.Usecases {
	repositories := repositories.NewRepositories(
		infra.InitializeFirebase(ctx),
		pool,
		infra.InitializeMetabase(appConfiguration.metabase),
		appConfiguration.config.GcsTransferCheckEnrichmentBucket,
	)

	return usecases.Usecases{
		Repositories:  *repositories,
		Configuration: appConfiguration.config,
	}
}
