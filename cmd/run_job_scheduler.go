package cmd

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/jobs"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/getsentry/sentry-go"
)

func RunJobScheduler() error {
	// This is where we read the environment variables and set up the configuration for the application.
	gcpConfig := infra.GcpConfig{
		EnableTracing: utils.GetEnv("ENABLE_GCP_TRACING", false),
		ProjectId:     utils.GetEnv("GOOGLE_CLOUD_PROJECT", ""),
	}
	pgConfig := infra.PgConfig{
		Database:            "marble",
		DbConnectWithSocket: utils.GetEnv("PG_CONNECT_WITH_SOCKET", false),
		Hostname:            utils.GetRequiredEnv[string]("PG_HOSTNAME"),
		Password:            utils.GetRequiredEnv[string]("PG_PASSWORD"),
		Port:                utils.GetEnv("PG_PORT", "5432"),
		User:                utils.GetRequiredEnv[string]("PG_USER"),
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
		fakeAwsS3Repository         bool
		failedWebhooksRetryPageSize int
		ingestionBucketUrl          string
	}{
		env:                         utils.GetEnv("ENV", "development"),
		appName:                     "marble-backend",
		ingestionBucketUrl:          utils.GetRequiredEnv[string]("INGESTION_BUCKET_URL"),
		loggingFormat:               utils.GetEnv("LOGGING_FORMAT", "text"),
		sentryDsn:                   utils.GetEnv("SENTRY_DSN", ""),
		fakeAwsS3Repository:         utils.GetEnv("FAKE_AWS_S3", false),
		failedWebhooksRetryPageSize: utils.GetEnv("FAILED_WEBHOOKS_RETRY_PAGE_SIZE", 1000),
	}

	logger := utils.NewLogger(jobConfig.loggingFormat)
	ctx := utils.StoreLoggerInContext(context.Background(), logger)
	license := infra.VerifyLicense(licenseConfig)

	infra.SetupSentry(jobConfig.sentryDsn, jobConfig.env)
	defer sentry.Flush(3 * time.Second)

	tracingConfig := infra.TelemetryConfiguration{
		ApplicationName: jobConfig.appName,
		Enabled:         gcpConfig.EnableTracing,
		ProjectID:       gcpConfig.ProjectId,
	}
	telemetryRessources, err := infra.InitTelemetry(tracingConfig)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return err
	}
	ctx = utils.StoreOpenTelemetryTracerInContext(ctx, telemetryRessources.Tracer)

	pool, err := infra.NewPostgresConnectionPool(ctx, pgConfig.GetConnectionString(), telemetryRessources.TracerProvider)
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
		),
	)
	uc := usecases.NewUsecases(repositories,
		usecases.WithGcsIngestionBucket(jobConfig.ingestionBucketUrl),
		usecases.WithFakeAwsS3Repository(jobConfig.fakeAwsS3Repository),
		usecases.WithFailedWebhooksRetryPageSize(jobConfig.failedWebhooksRetryPageSize),
		usecases.WithLicense(license))

	jobs.RunScheduler(ctx, uc)

	return nil
}
