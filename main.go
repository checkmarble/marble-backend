package main

import (
	"context"
	"crypto/rsa"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/getsentry/sentry-go"
	"github.com/segmentio/analytics-go/v3"

	"github.com/checkmarble/marble-backend/api"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/jobs"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/firebase"
	"github.com/checkmarble/marble-backend/repositories/postgres"
	"github.com/checkmarble/marble-backend/tracing"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/token"
	"github.com/checkmarble/marble-backend/utils"
)

type dependencies struct {
	Authentication      *api.Authentication
	TokenHandler        *api.TokenHandler
	SegmentClient       analytics.Client
	TelemetryRessources tracing.TelemetryRessources
}

func initDependencies(conf AppConfiguration, signingKey *rsa.PrivateKey) (dependencies, error) {
	database, err := postgres.New(postgres.Configuration{
		Host:                conf.pgConfig.Hostname,
		Port:                conf.pgConfig.Port,
		User:                conf.pgConfig.User,
		Password:            conf.pgConfig.Password,
		Database:            conf.pgConfig.Database,
		DbConnectWithSocket: conf.pgConfig.DbConnectWithSocket,
	})
	if err != nil {
		return dependencies{}, fmt.Errorf("postgres.New error: %w", err)
	}

	auth := infra.InitializeFirebase(context.Background())
	firebaseClient := firebase.New(auth)
	jwtRepository := repositories.NewJWTRepository(signingKey)
	tokenValidator := token.NewValidator(database, jwtRepository)
	tokenGenerator := token.NewGenerator(database, jwtRepository, firebaseClient, conf.config.TokenLifetimeMinute)
	segmentClient := analytics.New(conf.config.SegmentWriteKey)

	return dependencies{
		Authentication:      api.NewAuthentication(tokenValidator),
		SegmentClient:       segmentClient,
		TelemetryRessources: tracing.NoopTelemetry(),
		TokenHandler:        api.NewTokenHandler(tokenGenerator),
	}, nil
}

func runServer(ctx context.Context, appConfig AppConfiguration, tracingConfig tracing.Configuration) {
	marbleJwtSigningKey := infra.ParseOrGenerateSigningKey(ctx, appConfig.config.JwtSigningKey)

	uc := NewUseCases(ctx, appConfig, marbleJwtSigningKey)

	logger := utils.LoggerFromContext(ctx)

	////////////////////////////////////////////////////////////
	// Seed the database
	////////////////////////////////////////////////////////////
	seedUsecase := uc.NewSeedUseCase()
	marbleAdminEmail := appConfig.seedOrgConfig.CreateGlobalAdminEmail
	if marbleAdminEmail != "" {
		if err := seedUsecase.SeedMarbleAdmins(ctx, marbleAdminEmail); err != nil {
			panic(err)
		}
	}
	if appConfig.seedOrgConfig.CreateOrgName != "" {
		if err := seedUsecase.CreateOrgAndUser(ctx, models.InitOrgInput{
			OrgName:    appConfig.seedOrgConfig.CreateOrgName,
			AdminEmail: appConfig.seedOrgConfig.CreateOrgAdminEmail,
		}); err != nil {
			panic(err)
		}
	}

	deps, err := initDependencies(appConfig, marbleJwtSigningKey)
	if err != nil {
		panic(err)
	}

	deps.TelemetryRessources, err = tracing.Init(tracingConfig)
	if err != nil {
		panic(err)
	}

	router := initRouter(ctx, appConfig, deps)
	server := api.New(router, appConfig.port, appConfig.config, uc, deps.Authentication, deps.TokenHandler)

	notify, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("starting server", slog.String("port", appConfig.port))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
}

type AppConfiguration struct {
	appName             string
	env                 string
	port                string
	gcpProject          string
	enableGcpTracing    bool
	requestLoggingLevel string
	loggingFormat       string
	pgConfig            utils.PGConfig
	config              models.GlobalConfiguration
	sentryDsn           string
	metabase            models.MetabaseConfiguration
	seedOrgConfig       models.SeedOrgConfiguration
}

func main() {
	appConfig := AppConfiguration{
		appName:             "marble-backend",
		env:                 utils.GetEnv("ENV", "development"),
		port:                utils.GetRequiredEnv[string]("PORT"),
		gcpProject:          os.Getenv("GOOGLE_CLOUD_PROJECT"),
		enableGcpTracing:    utils.GetEnv("ENABLE_GCP_TRACING", false),
		requestLoggingLevel: utils.GetEnv("REQUEST_LOGGING_LEVEL", "all"),
		loggingFormat:       utils.GetEnv("LOGGING_FORMAT", "text"),
		pgConfig: utils.PGConfig{
			Database:            "marble",
			DbConnectWithSocket: utils.GetEnv("PG_CONNECT_WITH_SOCKET", false),
			Hostname:            utils.GetRequiredEnv[string]("PG_HOSTNAME"),
			Password:            utils.GetRequiredEnv[string]("PG_PASSWORD"),
			Port:                utils.GetEnv("PG_PORT", "5432"),
			User:                utils.GetRequiredEnv[string]("PG_USER"),
		},
		config: models.GlobalConfiguration{
			TokenLifetimeMinute:  utils.GetEnv("TOKEN_LIFETIME_MINUTE", 60*2),
			FakeAwsS3Repository:  utils.GetEnv("FAKE_AWS_S3", false),
			FakeGcsRepository:    utils.GetEnv("FAKE_GCS", false),
			GcsIngestionBucket:   utils.GetRequiredEnv[string]("GCS_INGESTION_BUCKET"),
			GcsCaseManagerBucket: utils.GetRequiredEnv[string]("GCS_CASE_MANAGER_BUCKET"),
			GcsTransferCheckEnrichmentBucket: utils.GetEnv("GCS_TRANSFER_CHECK_ENRICHMENT_BUCKET",
				"case-manager-tokyo-country-381508"), // TODO: fixme hard coded placeholder
			MarbleAppHost:        utils.GetEnv("MARBLE_APP_HOST", ""),
			MarbleBackofficeHost: utils.GetEnv("MARBLE_BACKOFFICE_HOST", ""),
			SegmentWriteKey:      utils.GetRequiredEnv[string]("SEGMENT_WRITE_KEY"),
			JwtSigningKey:        utils.GetEnv("AUTHENTICATION_JWT_SIGNING_KEY", ""),
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

	logger := utils.NewLogger(appConfig.loggingFormat)
	appContext := utils.StoreLoggerInContext(context.Background(), logger)

	shouldRunMigrations := flag.Bool("migrations", false, "Run migrations")
	shouldRunServer := flag.Bool("server", false, "Run server")
	shouldRunScheduleScenarios := flag.Bool("scheduler", false, "Run schedule scenarios")
	shouldRunExecuteScheduledScenarios := flag.Bool("scheduled-executer", false, "Run execute scheduled scenarios")
	shouldRunDataIngestion := flag.Bool("data-ingestion", false, "Run data ingestion")
	shouldRunScheduler := flag.Bool("cron-scheduler", false, "Run scheduler for cron jobs")
	flag.Parse()
	logger.InfoContext(appContext, "Flags",
		slog.Bool("shouldRunMigrations", *shouldRunMigrations),
		slog.Bool("shouldRunServer", *shouldRunServer),
		slog.Bool("shouldRunScheduledScenarios", *shouldRunScheduleScenarios),
		slog.Bool("shouldRunDataIngestion", *shouldRunDataIngestion),
		slog.Bool("shouldRunScheduler", *shouldRunScheduler),
	)

	setupSentry(appConfig)
	defer sentry.Flush(3 * time.Second)

	tracingConfig := tracing.Configuration{
		ApplicationName: appConfig.appName,
		Enabled:         appConfig.enableGcpTracing,
		ProjectID:       appConfig.gcpProject,
	}

	if *shouldRunMigrations {
		migrater := repositories.NewMigrater(appConfig.pgConfig)
		if err := migrater.Run(appContext); err != nil {
			logger.ErrorContext(appContext, fmt.Sprintf(
				"error while running migrations: %+v", err))
			os.Exit(1)
			return
		}
	}

	if *shouldRunServer {
		runServer(appContext, appConfig, tracingConfig)
	}

	if *shouldRunScheduleScenarios {
		usecases := NewUseCases(appContext, appConfig, nil)
		err := jobs.ScheduleDueScenarios(appContext, usecases, tracingConfig)
		if err != nil {
			logger.ErrorContext(appContext, "jobs.ScheduleDueScenarios failed", slog.String("error", err.Error()))
			os.Exit(1)
			return
		}
	}

	if *shouldRunExecuteScheduledScenarios {
		usecases := NewUseCases(appContext, appConfig, nil)
		err := jobs.ExecuteAllScheduledScenarios(appContext, usecases, tracingConfig)
		if err != nil {
			logger.ErrorContext(appContext, "jobs.ExecuteAllScheduledScenarios failed", slog.String("error", err.Error()))
			os.Exit(1)
			return
		}
	}

	if *shouldRunDataIngestion {
		usecases := NewUseCases(appContext, appConfig, nil)
		err := jobs.IngestDataFromCsv(appContext, usecases, tracingConfig)
		if err != nil {
			logger.ErrorContext(appContext, "jobs.IngestDataFromCsv failed", slog.String("error", err.Error()))
			os.Exit(1)
			return
		}
	}

	if *shouldRunScheduler {
		jobs.RunScheduler(appContext, NewUseCases(appContext, appConfig, nil), tracingConfig)
	}
}

func NewUseCases(ctx context.Context, appConfiguration AppConfiguration, marbleJwtSigningKey *rsa.PrivateKey) usecases.Usecases {
	marbleConnectionPool, err := infra.NewPostgresConnectionPool(
		appConfiguration.pgConfig.GetConnectionString())
	if err != nil {
		log.Fatal("error creating postgres connection to marble database", err.Error())
	}

	repositories, err := repositories.NewRepositories(
		marbleJwtSigningKey,
		infra.InitializeFirebase(ctx),
		marbleConnectionPool,
		infra.InitializeMetabase(appConfiguration.metabase),
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

func setupSentry(conf AppConfiguration) {
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:           conf.sentryDsn,
		EnableTracing: true,
		Environment:   conf.env,
		TracesSampler: sentry.TracesSampler(func(ctx sentry.SamplingContext) float64 {
			if ctx.Span.Name == "GET /liveness" {
				return 0.0
			}
			if ctx.Span.Name == "POST /ingestion/:object_type" {
				return 0.05
			}
			if ctx.Span.Name == "POST /decisions" {
				return 0.05
			}
			if ctx.Span.Name == "GET /token" {
				return 0.05
			}
			return 0.1
		}),
		// Experimental - value to be adjusted in prod once volumes go up - relative to the trace sampling rate
		ProfilesSampleRate: 0.2,
	}); err != nil {
		panic(err)
	}
}
