package main

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
	"github.com/cockroachdb/errors"
	"github.com/getsentry/sentry-go"
)

func runServer(ctx context.Context) error {
	// This is where we read the environment variables and set up the configuration for the application.
	apiConfig := api.Configuration{
		Env:                  utils.GetEnv("ENV", "development"),
		AppName:              "marble-backend",
		MarbleAppHost:        utils.GetEnv("MARBLE_APP_HOST", ""),
		MarbleBackofficeHost: utils.GetEnv("MARBLE_BACKOFFICE_HOST", ""),
		Port:                 utils.GetRequiredEnv[string]("PORT"),
		RequestLoggingLevel:  utils.GetEnv("REQUEST_LOGGING_LEVEL", "all"),
		TokenLifetimeMinute:  utils.GetEnv("TOKEN_LIFETIME_MINUTE", 60*2),
		SegmentWriteKey:      utils.GetEnv("SEGMENT_WRITE_KEY", ""),
	}
	gcpConfig := infra.GcpConfig{
		FakeGcsRepository:                utils.GetEnv("FAKE_GCS", false),
		EnableTracing:                    utils.GetEnv("ENABLE_GCP_TRACING", false),
		ProjectId:                        utils.GetEnv("GOOGLE_CLOUD_PROJECT", ""),
		GcsIngestionBucket:               utils.GetEnv("GCS_INGESTION_BUCKET", ""),
		GcsCaseManagerBucket:             utils.GetEnv("GCS_CASE_MANAGER_BUCKET", ""),
		GcsTransferCheckEnrichmentBucket: utils.GetEnv("GCS_TRANSFER_CHECK_ENRICHMENT_BUCKET", ""), // required for transfercheck
	}
	pgConfig := infra.PgConfig{
		Database:            "marble",
		DbConnectWithSocket: utils.GetEnv("PG_CONNECT_WITH_SOCKET", false),
		Hostname:            utils.GetRequiredEnv[string]("PG_HOSTNAME"),
		Password:            utils.GetRequiredEnv[string]("PG_PASSWORD"),
		Port:                utils.GetEnv("PG_PORT", "5432"),
		User:                utils.GetRequiredEnv[string]("PG_USER"),
	}
	metabaseConfig := infra.MetabaseConfiguration{
		SiteUrl:             utils.GetEnv("METABASE_SITE_URL", ""),
		JwtSigningKey:       []byte(utils.GetEnv("METABASE_JWT_SIGNING_KEY", "")),
		TokenLifetimeMinute: utils.GetEnv("METABASE_TOKEN_LIFETIME_MINUTE", 10),
		Resources: map[models.EmbeddingType]int{
			models.GlobalDashboard: utils.GetEnv("METABASE_GLOBAL_DASHBOARD_ID", 0),
		},
	}
	seedOrgConfig := models.SeedOrgConfiguration{
		CreateGlobalAdminEmail: utils.GetEnv("CREATE_GLOBAL_ADMIN_EMAIL", ""),
		CreateOrgName:          utils.GetEnv("CREATE_ORG_NAME", ""),
		CreateOrgAdminEmail:    utils.GetEnv("CREATE_ORG_ADMIN_EMAIL", ""),
	}
	serverConfig := struct {
		jwtSigningKey string
		loggingFormat string
		sentryDsn     string
	}{
		jwtSigningKey: utils.GetEnv("AUTHENTICATION_JWT_SIGNING", ""),
		loggingFormat: utils.GetEnv("LOGGING_FORMAT", "text"),
		sentryDsn:     utils.GetEnv("SENTRY_DSN", ""),
	}

	marbleJwtSigningKey := infra.ParseOrGenerateSigningKey(ctx, serverConfig.jwtSigningKey)
	logger := utils.NewLogger(serverConfig.loggingFormat)

	infra.SetupSentry(serverConfig.sentryDsn, apiConfig.Env)
	defer sentry.Flush(3 * time.Second)

	tracingConfig := infra.TelemetryConfiguration{
		ApplicationName: apiConfig.AppName,
		Enabled:         gcpConfig.EnableTracing,
		ProjectID:       gcpConfig.ProjectId,
	}
	telemetryRessources, err := infra.InitTelemetry(tracingConfig)
	if err != nil {
		return err
	}

	marbleConnectionPool, err := infra.NewPostgresConnectionPool(ctx, pgConfig.GetConnectionString())
	if err != nil {
		return err
	}

	repositories := repositories.NewRepositories(
		infra.InitializeFirebase(ctx),
		marbleConnectionPool,
		infra.InitializeMetabase(metabaseConfig),
		gcpConfig.GcsTransferCheckEnrichmentBucket,
	)

	uc := usecases.NewUsecases(repositories,
		usecases.WithFakeGcsRepository(gcpConfig.FakeGcsRepository),
		usecases.WithGcsIngestionBucket(gcpConfig.GcsIngestionBucket),
		usecases.WithGcsCaseManagerBucket(gcpConfig.GcsCaseManagerBucket),
	)

	////////////////////////////////////////////////////////////
	// Seed the database
	////////////////////////////////////////////////////////////
	seedUsecase := uc.NewSeedUseCase()
	marbleAdminEmail := seedOrgConfig.CreateGlobalAdminEmail
	if marbleAdminEmail != "" {
		if err := seedUsecase.SeedMarbleAdmins(ctx, marbleAdminEmail); err != nil {
			return err
		}
	}
	if seedOrgConfig.CreateOrgName != "" {
		if err := seedUsecase.CreateOrgAndUser(ctx, models.InitOrgInput{
			OrgName:    seedOrgConfig.CreateOrgName,
			AdminEmail: seedOrgConfig.CreateOrgAdminEmail,
		}); err != nil {
			return err
		}
	}

	deps := api.InitDependencies(ctx, apiConfig, marbleConnectionPool, marbleJwtSigningKey)

	router := api.InitRouter(ctx, apiConfig, deps.SegmentClient, telemetryRessources)
	server := api.New(router, apiConfig.Port, apiConfig.MarbleAppHost, uc, deps.Authentication, deps.TokenHandler)

	notify, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.InfoContext(ctx, "starting server", slog.String("port", apiConfig.Port))
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
