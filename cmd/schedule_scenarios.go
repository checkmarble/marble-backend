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

func RunScheduleScenarios() error {
	// This is where we read the environment variables and set up the configuration for the application.
	gcpConfig := infra.GcpConfig{
		EnableTracing:    utils.GetEnv("ENABLE_GCP_TRACING", false),
		TracingProjectId: utils.GetEnv("GOOGLE_CLOUD_PROJECT", ""),
	}
	pgConfig := infra.PgConfig{
		Database:            "marble",
		DbConnectWithSocket: utils.GetEnv("PG_CONNECT_WITH_SOCKET", false),
		Hostname:            utils.GetRequiredEnv[string]("PG_HOSTNAME"),
		Password:            utils.GetRequiredEnv[string]("PG_PASSWORD"),
		Port:                utils.GetEnv("PG_PORT", "5432"),
		User:                utils.GetRequiredEnv[string]("PG_USER"),
	}
	licenseConfig := models.LicenseConfiguration{
		LicenseKey:             utils.GetEnv("LICENSE_KEY", ""),
		KillIfReadLicenseError: utils.GetEnv("KILL_IF_READ_LICENSE_ERROR", false),
	}
	jobConfig := struct {
		env           string
		appName       string
		loggingFormat string
		sentryDsn     string
	}{
		env:           utils.GetEnv("ENV", "development"),
		appName:       "marble-backend",
		loggingFormat: utils.GetEnv("LOGGING_FORMAT", "text"),
		sentryDsn:     utils.GetEnv("SENTRY_DSN", ""),
	}

	logger := utils.NewLogger(jobConfig.loggingFormat)
	ctx := utils.StoreLoggerInContext(context.Background(), logger)
	license := infra.VerifyLicense(licenseConfig)

	infra.SetupSentry(jobConfig.sentryDsn, jobConfig.env)
	defer sentry.Flush(3 * time.Second)

	tracingConfig := infra.TelemetryConfiguration{
		ApplicationName: jobConfig.appName,
		Enabled:         gcpConfig.EnableTracing,
		ProjectID:       gcpConfig.TracingProjectId,
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

	repositories := repositories.NewRepositories(pool)
	uc := usecases.NewUsecases(repositories,
		usecases.WithLicense(license))

	err = jobs.ScheduleDueScenarios(ctx, uc)
	if err != nil {
		logger.ErrorContext(ctx, "failed to schedule scenarios", slog.String("error", err.Error()))
	}

	return err
}
