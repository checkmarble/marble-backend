package cmd

import (
	"context"
	"log/slog"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/jobs"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/getsentry/sentry-go"
)

// Deprecated
func RunSendPendingWebhookEvents(apiVersion string) error {
	// This is where we read the environment variables and set up the configuration for the application.
	gcpConfig := infra.GcpConfig{
		EnableTracing: utils.GetEnv("ENABLE_GCP_TRACING", false),
		ProjectId:     utils.GetEnv("GOOGLE_CLOUD_PROJECT", ""),
	}
	pgConfig := infra.PgConfig{
		ConnectionString:   utils.GetEnv("PG_CONNECTION_STRING", ""),
		Database:           utils.GetEnv("PG_DATABASE", "marble"),
		Hostname:           utils.GetEnv("PG_HOSTNAME", ""),
		Password:           utils.GetEnv("PG_PASSWORD", ""),
		Port:               utils.GetEnv("PG_PORT", "5432"),
		User:               utils.GetEnv("PG_USER", ""),
		MaxPoolConnections: utils.GetEnv("PG_MAX_POOL_SIZE", infra.DEFAULT_MAX_CONNECTIONS),
		SslMode:            utils.GetEnv("PG_SSL_MODE", "prefer"),
	}
	convoyConfiguration := infra.ConvoyConfiguration{
		APIKey:    utils.GetEnv("CONVOY_API_KEY", ""),
		APIUrl:    utils.GetEnv("CONVOY_API_URL", ""),
		ProjectID: utils.GetEnv("CONVOY_PROJECT_ID", ""),
		RateLimit: utils.GetEnv("CONVOY_RATE_LIMIT", 50),
	}
	licenseConfig := models.LicenseConfiguration{
		LicenseKey:             utils.GetEnv("LICENSE_KEY", ""),
		KillIfReadLicenseError: utils.GetEnv("KILL_IF_READ_LICENSE_ERROR", false),
	}
	jobConfig := struct {
		env                         string
		appName                     string
		loggingFormat               string
		sentryDsn                   string
		failedWebhooksRetryPageSize int
	}{
		env:                         utils.GetEnv("ENV", "development"),
		appName:                     "marble-backend",
		loggingFormat:               utils.GetEnv("LOGGING_FORMAT", "text"),
		sentryDsn:                   utils.GetEnv("SENTRY_DSN", ""),
		failedWebhooksRetryPageSize: utils.GetEnv("FAILED_WEBHOOKS_RETRY_PAGE_SIZE", 1000),
	}

	logger := utils.NewLogger(jobConfig.loggingFormat)
	ctx := utils.StoreLoggerInContext(context.Background(), logger)
	license := infra.VerifyLicense(licenseConfig)

	infra.SetupSentry(jobConfig.sentryDsn, jobConfig.env, apiVersion)
	defer sentry.Flush(3 * time.Second)

	tracingConfig := infra.TelemetryConfiguration{
		ApplicationName: jobConfig.appName,
		Enabled:         gcpConfig.EnableTracing,
		ProjectID:       gcpConfig.ProjectId,
	}
	telemetryRessources, err := infra.InitTelemetry(tracingConfig, apiVersion)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return err
	}
	ctx = utils.StoreOpenTelemetryTracerInContext(ctx, telemetryRessources.Tracer)

	pool, err := infra.NewPostgresConnectionPool(ctx, pgConfig.GetConnectionString(),
		telemetryRessources.TracerProvider, pgConfig.MaxPoolConnections)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return err
	}

	repositories := repositories.NewRepositories(
		pool,
		gcpConfig.GoogleApplicationCredentials,
		repositories.WithConvoyClientProvider(
			infra.InitializeConvoyRessources(convoyConfiguration),
			convoyConfiguration.RateLimit,
		))
	uc := usecases.NewUsecases(repositories,
		usecases.WithFailedWebhooksRetryPageSize(jobConfig.failedWebhooksRetryPageSize),
		usecases.WithLicense(license),
		usecases.WithConvoyServer(convoyConfiguration.APIUrl),
	)

	logger.InfoContext(ctx, "starting pending webhook event sender", slog.String("version", apiVersion))

	jobs.SendPendingWebhookEvents(ctx, uc)
	return nil
}
