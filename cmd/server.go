package cmd

import (
	"context"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/checkmarble/marble-backend/api"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/cockroachdb/errors"
	"github.com/getsentry/sentry-go"
)

func RunServer(config CompiledConfig) error {
	// This is where we read the environment variables and set up the configuration for the application.
	apiConfig := api.Configuration{
		Env:                 utils.GetEnv("ENV", "development"),
		AppName:             "marble-backend",
		MarbleAppUrl:        utils.GetEnv("MARBLE_APP_URL", ""),
		MarbleBackofficeUrl: utils.GetEnv("MARBLE_BACKOFFICE_URL", ""),
		Port:                utils.GetRequiredEnv[string]("PORT"),
		RequestLoggingLevel: utils.GetEnv("REQUEST_LOGGING_LEVEL", "all"),
		TokenLifetimeMinute: utils.GetEnv("TOKEN_LIFETIME_MINUTE", 60*2),
		SegmentWriteKey:     utils.GetEnv("SEGMENT_WRITE_KEY", config.SegmentWriteKey),
		DisableSegment:      utils.GetEnv("DISABLE_SEGMENT", false),
		BatchTimeout:        time.Duration(utils.GetEnv("BATCH_TIMEOUT_SECOND", 55)) * time.Second,
		DecisionTimeout:     time.Duration(utils.GetEnv("DECISION_TIMEOUT_SECOND", 10)) * time.Second,
		DefaultTimeout:      time.Duration(utils.GetEnv("DEFAULT_TIMEOUT_SECOND", 5)) * time.Second,
	}
	if apiConfig.DisableSegment {
		apiConfig.SegmentWriteKey = ""
	}
	gcpConfig := infra.GcpConfig{
		EnableTracing:                utils.GetEnv("ENABLE_GCP_TRACING", false),
		ProjectId:                    utils.GetEnv("GOOGLE_CLOUD_PROJECT", ""),
		GoogleApplicationCredentials: utils.GetEnv("GOOGLE_APPLICATION_CREDENTIALS", ""),
	}
	pgConfig := infra.PgConfig{
		ConnectionString:   utils.GetEnv("PG_CONNECTION_STRING", ""),
		Database:           utils.GetEnv("PG_DATABASE", "marble"),
		Hostname:           utils.GetEnv("PG_HOSTNAME", ""),
		Password:           utils.GetEnv("PG_PASSWORD", ""),
		Port:               utils.GetEnv("PG_PORT", "5432"),
		User:               utils.GetEnv("PG_USER", ""),
		MaxPoolConnections: utils.GetEnv("PG_MAX_POOL_SIZE", infra.DEFAULT_MAX_CONNECTIONS),
		ClientDbConfigFile: utils.GetEnv("CLIENT_DB_CONFIG_FILE", ""),
		SslMode:            utils.GetEnv("PG_SSL_MODE", "prefer"),
	}
	metabaseConfig := infra.MetabaseConfiguration{
		SiteUrl:             utils.GetEnv("METABASE_SITE_URL", ""),
		JwtSigningKey:       []byte(utils.GetEnv("METABASE_JWT_SIGNING_KEY", "")),
		TokenLifetimeMinute: utils.GetEnv("METABASE_TOKEN_LIFETIME_MINUTE", 10),
		Resources: map[models.EmbeddingType]int{
			models.GlobalDashboard: utils.GetEnv("METABASE_GLOBAL_DASHBOARD_ID", 0),
		},
	}
	convoyConfiguration := infra.ConvoyConfiguration{
		APIKey:    utils.GetEnv("CONVOY_API_KEY", ""),
		APIUrl:    utils.GetEnv("CONVOY_API_URL", ""),
		ProjectID: utils.GetEnv("CONVOY_PROJECT_ID", ""),
		RateLimit: utils.GetEnv("CONVOY_RATE_LIMIT", 50),
	}

	openSanctionsConfig := infra.InitializeOpenSanctions(
		http.DefaultClient,
		utils.GetEnv("OPENSANCTIONS_API_HOST", ""),
		utils.GetEnv("OPENSANCTIONS_AUTH_METHOD", ""),
		utils.GetEnv("OPENSANCTIONS_API_KEY", ""),
	)

	if apiUrl := utils.GetEnv("NAME_RECOGNITION_API_URL", ""); apiUrl != "" {
		openSanctionsConfig.WithNameRecognition(apiUrl,
			utils.GetEnv("NAME_RECOGNITION_API_KEY", ""))
	}

	seedOrgConfig := models.SeedOrgConfiguration{
		CreateGlobalAdminEmail: utils.GetEnv("CREATE_GLOBAL_ADMIN_EMAIL", ""),
		CreateOrgName:          utils.GetEnv("CREATE_ORG_NAME", ""),
		CreateOrgAdminEmail:    utils.GetEnv("CREATE_ORG_ADMIN_EMAIL", ""),
	}
	licenseConfig := models.LicenseConfiguration{
		LicenseKey:             utils.GetEnv("LICENSE_KEY", ""),
		KillIfReadLicenseError: utils.GetEnv("KILL_IF_READ_LICENSE_ERROR", false),
	}
	serverConfig := struct {
		batchIngestionMaxSize            int
		caseManagerBucket                string
		ingestionBucketUrl               string
		offloadingBucketUrl              string
		jwtSigningKey                    string
		jwtSigningKeyFile                string
		loggingFormat                    string
		sentryDsn                        string
		transferCheckEnrichmentBucketUrl string
		firebaseEmulatorHost             string
	}{
		batchIngestionMaxSize:            utils.GetEnv("BATCH_INGESTION_MAX_SIZE", 0),
		caseManagerBucket:                utils.GetEnv("CASE_MANAGER_BUCKET_URL", ""),
		ingestionBucketUrl:               utils.GetEnv("INGESTION_BUCKET_URL", ""),
		offloadingBucketUrl:              utils.GetEnv("OFFLOADING_BUCKET_URL", ""),
		jwtSigningKey:                    utils.GetEnv("AUTHENTICATION_JWT_SIGNING_KEY", ""),
		jwtSigningKeyFile:                utils.GetEnv("AUTHENTICATION_JWT_SIGNING_KEY_FILE", ""),
		loggingFormat:                    utils.GetEnv("LOGGING_FORMAT", "text"),
		sentryDsn:                        utils.GetEnv("SENTRY_DSN", ""),
		transferCheckEnrichmentBucketUrl: utils.GetEnv("TRANSFER_CHECK_ENRICHMENT_BUCKET_URL", ""), // required for transfercheck
		firebaseEmulatorHost:             utils.GetEnv("FIREBASE_AUTH_EMULATOR_HOST", ""),
	}

	logger := utils.NewLogger(serverConfig.loggingFormat)

	ctx := utils.StoreLoggerInContext(context.Background(), logger)
	marbleJwtSigningKey := infra.ReadParseOrGenerateSigningKey(ctx, serverConfig.jwtSigningKey, serverConfig.jwtSigningKeyFile)
	license := infra.VerifyLicense(licenseConfig)

	infra.SetupSentry(serverConfig.sentryDsn, apiConfig.Env, config.Version)
	defer sentry.Flush(3 * time.Second)

	tracingConfig := infra.TelemetryConfiguration{
		ApplicationName: apiConfig.AppName,
		Enabled:         gcpConfig.EnableTracing,
		ProjectID:       gcpConfig.ProjectId,
	}
	telemetryRessources, err := infra.InitTelemetry(tracingConfig, config.Version)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
	}

	pool, err := infra.NewPostgresConnectionPool(ctx, pgConfig.GetConnectionString(),
		telemetryRessources.TracerProvider, pgConfig.MaxPoolConnections)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
	}

	clientDbConfig, err := infra.ParseClientDbConfig(pgConfig.ClientDbConfigFile)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return err
	}

	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return err
	}

	repositories := repositories.NewRepositories(
		pool,
		gcpConfig.GoogleApplicationCredentials,
		repositories.WithMetabase(infra.InitializeMetabase(metabaseConfig)),
		repositories.WithTransferCheckEnrichmentBucket(serverConfig.transferCheckEnrichmentBucketUrl),
		repositories.WithConvoyClientProvider(
			infra.InitializeConvoyRessources(convoyConfiguration),
			convoyConfiguration.RateLimit,
		),
		repositories.WithOpenSanctions(openSanctionsConfig),
		repositories.WithClientDbConfig(clientDbConfig),
		repositories.WithTracerProvider(telemetryRessources.TracerProvider),
		repositories.WithRiverClient(riverClient),
	)

	uc := usecases.NewUsecases(repositories,
		usecases.WithApiVersion(config.Version),
		usecases.WithBatchIngestionMaxSize(serverConfig.batchIngestionMaxSize),
		usecases.WithIngestionBucketUrl(serverConfig.ingestionBucketUrl),
		usecases.WithOffloadingBucketUrl(serverConfig.offloadingBucketUrl),
		usecases.WithCaseManagerBucketUrl(serverConfig.caseManagerBucket),
		usecases.WithLicense(license),
		usecases.WithConvoyServer(convoyConfiguration.APIUrl),
		usecases.WithMetabase(metabaseConfig.SiteUrl),
		usecases.WithOpensanctions(openSanctionsConfig.IsSet()),
		usecases.WithNameRecognition(openSanctionsConfig.IsNameRecognitionSet()),
		usecases.WithTestMode(serverConfig.firebaseEmulatorHost != ""),
	)

	////////////////////////////////////////////////////////////
	// Seed the database
	////////////////////////////////////////////////////////////
	seedUsecase := uc.NewSeedUseCase()
	marbleAdminEmail := seedOrgConfig.CreateGlobalAdminEmail
	if marbleAdminEmail != "" {
		if err := seedUsecase.SeedMarbleAdmins(ctx, marbleAdminEmail); err != nil {
			utils.LogAndReportSentryError(ctx, err)
			return err
		}
	}
	if seedOrgConfig.CreateOrgName != "" {
		if err := seedUsecase.CreateOrgAndUser(ctx, models.InitOrgInput{
			OrgName:    seedOrgConfig.CreateOrgName,
			AdminEmail: seedOrgConfig.CreateOrgAdminEmail,
		}); err != nil {
			utils.LogAndReportSentryError(ctx, err)
			return err
		}
	}

	deps := api.InitDependencies(ctx, apiConfig, pool, marbleJwtSigningKey)

	router := api.InitRouterMiddlewares(ctx, apiConfig, apiConfig.DisableSegment, deps.SegmentClient, telemetryRessources)
	server := api.NewServer(router, apiConfig, uc, deps.Authentication, deps.TokenHandler, logger)

	notify, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.InfoContext(ctx, "starting server", slog.String("version", config.Version), slog.String("port", apiConfig.Port))
		err := server.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			utils.LogAndReportSentryError(ctx, errors.Wrap(err, "Error while serving the app"))
		}
		logger.InfoContext(ctx, "server returned")
	}()

	<-notify.Done()
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	deps.SegmentClient.Close()

	if err := server.Shutdown(shutdownCtx); err != nil {
		utils.LogAndReportSentryError(
			ctx,
			errors.Wrap(err, "Error while shutting down the server"),
		)
		return err
	}

	return err
}
