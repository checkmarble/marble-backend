package cmd

import (
	"context"
	"fmt"
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
	"github.com/checkmarble/marble-backend/usecases/auth"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/cockroachdb/errors"
	"github.com/getsentry/sentry-go"
)

func RunServer(config CompiledConfig) error {
	appName := fmt.Sprintf("marble-backend %s", config.Version)
	logger := utils.NewLogger(utils.GetEnv("LOGGING_FORMAT", "text"))
	ctx := utils.StoreLoggerInContext(context.Background(), logger)

	isMarbleSaasProject := infra.IsMarbleSaasProject()

	authProvider := auth.ParseTokenProvider(utils.GetEnv("AUTH_PROVIDER", "firebase"))
	oidcProvider := infra.OidcConfig{}

	if authProvider == auth.TokenProviderOidc {
		oidc, err := infra.InitializeOidc(ctx)
		if err != nil {
			return err
		}

		oidcProvider = oidc
	}

	// This is where we read the environment variables and set up the configuration for the application.
	apiConfig := api.Configuration{
		Env:                 utils.GetEnv("ENV", "development"),
		AppName:             "marble-backend",
		MarbleApiUrl:        utils.GetEnv("MARBLE_API_URL", ""),
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
		AnalyticsTimeout:    utils.GetEnvDuration("ANALYTICS_TIMEOUT", 15*time.Second),

		MetabaseConfig: infra.MetabaseConfiguration{
			SiteUrl:             utils.GetEnv("METABASE_SITE_URL", ""),
			JwtSigningKey:       []byte(utils.GetEnv("METABASE_JWT_SIGNING_KEY", "")),
			TokenLifetimeMinute: utils.GetEnv("METABASE_TOKEN_LIFETIME_MINUTE", 10),
			Resources: map[models.EmbeddingType]int{
				models.GlobalDashboard: utils.GetEnv("METABASE_GLOBAL_DASHBOARD_ID", 0),
			},
		},

		TokenProvider: authProvider,
		FirebaseConfig: api.FirebaseConfig{
			ProjectId:    utils.GetEnv("FIREBASE_PROJECT_ID", ""),
			EmulatorHost: utils.GetEnv("FIREBASE_AUTH_EMULATOR_HOST", ""),
			ApiKey:       utils.GetRequiredEnv[string]("FIREBASE_API_KEY"),
			AuthDomain:   utils.GetEnv("FIREBASE_AUTH_DOMAIN", ""),
		},
		OidcConfig: oidcProvider,
	}
	if apiConfig.DisableSegment {
		apiConfig.SegmentWriteKey = ""
	}

	gcpConfig, err := infra.NewGcpConfig(
		ctx,
		utils.GetEnv("GOOGLE_CLOUD_PROJECT", ""),
		utils.GetEnv("GOOGLE_APPLICATION_CREDENTIALS", ""),
		utils.GetEnv("ENABLE_GCP_TRACING", false),
	)
	if err != nil {
		return err
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
		ImpersonateRole:    utils.GetEnv("PG_IMPERSONATE_ROLE", ""),
	}
	convoyConfiguration := infra.ConvoyConfiguration{
		APIKey:    utils.GetEnv("CONVOY_API_KEY", ""),
		APIUrl:    utils.GetEnv("CONVOY_API_URL", ""),
		ProjectID: utils.GetEnv("CONVOY_PROJECT_ID", ""),
		RateLimit: utils.GetEnv("CONVOY_RATE_LIMIT", 50),
	}

	openSanctionsConfig := infra.InitializeOpenSanctions(
		otelhttp.DefaultClient,
		utils.GetEnv("OPENSANCTIONS_API_HOST", ""),
		utils.GetEnv("OPENSANCTIONS_AUTH_METHOD", ""),
		utils.GetEnv("OPENSANCTIONS_API_KEY", ""),
	)

	if scope := utils.GetEnv("OPENSANCTIONS_SCOPE", ""); scope != "" {
		openSanctionsConfig.WithScope(scope)
	}
	if algo := utils.GetEnv("OPENSANCTIONS_ALGORITHM", ""); algo != "" {
		openSanctionsConfig.WithAlgorithm(algo)
	}

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
	bigQueryConfig := infra.BigQueryConfig{
		ProjectID:      utils.GetEnv("BIGQUERY_PROJECT_ID", gcpConfig.ProjectId),
		MetricsDataset: utils.GetEnv("BIGQUERY_METRICS_DATASET", infra.MetricsDataset),
		MetricsTable:   utils.GetEnv("BIGQUERY_METRICS_TABLE", infra.MetricsTable),
	}
	aiAgentConfig := infra.AIAgentConfiguration{
		MainAgentProviderType: infra.AIAgentProviderTypeFromString(
			utils.GetEnv("AI_AGENT_MAIN_AGENT_PROVIDER_TYPE", "openai"),
		),
		MainAgentURL:          utils.GetEnv("AI_AGENT_MAIN_AGENT_URL", ""),
		MainAgentKey:          utils.GetEnv("AI_AGENT_MAIN_AGENT_KEY", ""),
		MainAgentDefaultModel: utils.GetEnv("AI_AGENT_MAIN_AGENT_DEFAULT_MODEL", "gemini-2.5-flash"),
		MainAgentBackend: infra.AIAgentProviderBackendFromString(
			utils.GetEnv("AI_AGENT_MAIN_AGENT_BACKEND", ""),
		),
		MainAgentProject:  utils.GetEnv("AI_AGENT_MAIN_AGENT_PROJECT", gcpConfig.ProjectId),
		MainAgentLocation: utils.GetEnv("AI_AGENT_MAIN_AGENT_LOCATION", ""),
		PerplexityAPIKey:  utils.GetEnv("AI_AGENT_PERPLEXITY_API_KEY", ""),
	}

	serverConfig := ServerConfig{
		batchIngestionMaxSize:            utils.GetEnv("BATCH_INGESTION_MAX_SIZE", 0),
		caseManagerBucket:                utils.GetEnv("CASE_MANAGER_BUCKET_URL", ""),
		ingestionBucketUrl:               utils.GetEnv("INGESTION_BUCKET_URL", ""),
		offloadingBucketUrl:              utils.GetEnv("OFFLOADING_BUCKET_URL", ""),
		analyticsBucketUrl:               utils.GetEnv("ANALYTICS_BUCKET_URL", ""),
		jwtSigningKey:                    utils.GetEnv("AUTHENTICATION_JWT_SIGNING_KEY", ""),
		jwtSigningKeyFile:                utils.GetEnv("AUTHENTICATION_JWT_SIGNING_KEY_FILE", ""),
		sentryDsn:                        utils.GetEnv("SENTRY_DSN", ""),
		transferCheckEnrichmentBucketUrl: utils.GetEnv("TRANSFER_CHECK_ENRICHMENT_BUCKET_URL", ""), // required for transfercheck
		telemetryExporter:                utils.GetEnv("TRACING_EXPORTER", "otlp"),
		otelSamplingRates:                utils.GetEnv("TRACING_SAMPLING_RATES", ""),
		similarityThreshold:              utils.GetEnv("SIMILARITY_THRESHOLD", models.DEFAULT_SIMILARITY_THRESHOLD),
	}
	if err := serverConfig.Validate(); err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return err
	}

	marbleJwtSigningKey := infra.ReadParseOrGenerateSigningKey(ctx, serverConfig.jwtSigningKey, serverConfig.jwtSigningKeyFile)

	logger.Info("successfully authenticated in GCP", "principal", gcpConfig.PrincipalEmail, "project", gcpConfig.ProjectId)

	if apiConfig.FirebaseConfig.ProjectId == "" {
		logger.Info("FIREBASE_PROJECT_ID was not provided, falling back to Google Cloud project", "project", gcpConfig.ProjectId)

		apiConfig.FirebaseConfig.ProjectId = gcpConfig.ProjectId
	}

	if !apiConfig.FirebaseConfig.IsEmulator() {
		if apiConfig.FirebaseConfig.ApiKey == "" {
			logger.Warn("no FIREBASE_API_KEY specified, this will be an error in the future")
		}

		if apiConfig.FirebaseConfig.AuthDomain == "" {
			apiConfig.FirebaseConfig.AuthDomain = fmt.Sprintf("%s.firebaseapp.com", apiConfig.FirebaseConfig.ProjectId)
			logger.Warn(fmt.Sprintf("no FIREBASE_AUTH_DOMAIN specified, defaulting to %s", apiConfig.FirebaseConfig.AuthDomain))
		}
	} else {
		// The auth domain, when using the emulator, is always the emulator host itself
		apiConfig.FirebaseConfig.AuthDomain = apiConfig.FirebaseConfig.EmulatorHost
	}

	logger.Info("firebase project configured", "project", apiConfig.FirebaseConfig.ProjectId)

	infra.SetupSentry(serverConfig.sentryDsn, apiConfig.Env, config.Version)
	defer sentry.Flush(3 * time.Second)

	tracingConfig := infra.TelemetryConfiguration{
		ApplicationName: apiConfig.AppName,
		Enabled:         gcpConfig.EnableTracing,
		ProjectID:       gcpConfig.ProjectId,
		Exporter:        serverConfig.telemetryExporter,
		SamplingMap:     infra.NewTelemetrySamplingMap(ctx, serverConfig.otelSamplingRates),
	}
	telemetryRessources, err := infra.InitTelemetry(tracingConfig, config.Version)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
	}

	pool, err := infra.NewPostgresConnectionPool(ctx, appName, pgConfig.GetConnectionString(),
		telemetryRessources.TracerProvider, pgConfig.MaxPoolConnections, pgConfig.ImpersonateRole)
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

	var bigQueryInfra *infra.BigQueryInfra
	if isMarbleSaasProject {
		bigQueryInfra, err = infra.InitializeBigQueryInfra(ctx, bigQueryConfig)
		if err != nil {
			utils.LogAndReportSentryError(ctx, err)
			return err
		}
		defer bigQueryInfra.Close()
	}

	var analyticsConfig infra.AnalyticsConfig

	if serverConfig.analyticsBucketUrl != "" {
		analyticsConfig, err = infra.InitAnalyticsConfig(pgConfig, serverConfig.analyticsBucketUrl)
		if err != nil {
			return err
		}

		apiConfig.AnalyticsEnabled = true
	}

	var lagoConfig infra.LagoConfig
	if isMarbleSaasProject {
		lagoConfig = infra.InitializeLago()
		if err := lagoConfig.Validate(); err != nil {
			utils.LogAndReportSentryError(ctx, err)
			return err
		}
	}

	repositories := repositories.NewRepositories(
		pool,
		gcpConfig,
		repositories.WithMetabase(infra.InitializeMetabase(apiConfig.MetabaseConfig)),
		repositories.WithTransferCheckEnrichmentBucket(serverConfig.transferCheckEnrichmentBucketUrl),
		repositories.WithConvoyClientProvider(
			infra.InitializeConvoyRessources(convoyConfiguration),
			convoyConfiguration.RateLimit,
		),
		repositories.WithOpenSanctions(openSanctionsConfig),
		repositories.WithClientDbConfig(clientDbConfig),
		repositories.WithTracerProvider(telemetryRessources.TracerProvider),
		repositories.WithRiverClient(riverClient),
		repositories.WithBigQueryInfra(bigQueryInfra),
		repositories.WithCache(utils.GetEnv("CACHE_ENABLED", false)),
		repositories.WithSimilarityThreshold(serverConfig.similarityThreshold),
		repositories.WithLagoConfig(lagoConfig),
	)

	deps, err := api.InitDependencies(ctx, apiConfig, pool, marbleJwtSigningKey)
	if err != nil {
		return err
	}

	deploymentMetadata, err := GetDeploymentMetadata(ctx, repositories)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return errors.Wrap(err, "failed to get deployment ID from Marble DB")
	}
	license := infra.VerifyLicense(licenseConfig, deploymentMetadata.Value)

	if apiConfig.TokenProvider == auth.TokenProviderOidc && !license.Sso {
		return errors.New("cannot use OpenID Connect configuration without the appropriate license entitlement")
	}

	uc := usecases.NewUsecases(repositories,
		usecases.WithAppName(appName),
		usecases.WithApiVersion(config.Version),
		usecases.WithBatchIngestionMaxSize(serverConfig.batchIngestionMaxSize),
		usecases.WithIngestionBucketUrl(serverConfig.ingestionBucketUrl),
		usecases.WithOffloadingBucketUrl(serverConfig.offloadingBucketUrl),
		usecases.WithCaseManagerBucketUrl(serverConfig.caseManagerBucket),
		usecases.WithLicense(license),
		usecases.WithConvoyServer(convoyConfiguration.APIUrl),
		usecases.WithMetabase(apiConfig.MetabaseConfig.SiteUrl),
		usecases.WithOpensanctions(openSanctionsConfig.IsSet()),
		usecases.WithNameRecognition(openSanctionsConfig.IsNameRecognitionSet()),
		usecases.WithFirebaseAdmin(apiConfig.TokenProvider, deps.FirebaseAdmin),
		usecases.WithAIAgentConfig(aiAgentConfig),
		usecases.WithAnalyticsConfig(analyticsConfig),
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

	router := api.InitRouterMiddlewares(ctx, apiConfig, apiConfig.DisableSegment,
		deps.SegmentClient, telemetryRessources)
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
