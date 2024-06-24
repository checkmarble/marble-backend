package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/jobs"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/getsentry/sentry-go"
)

func runScheduledExecuter(ctx context.Context) error {
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
	jobConfig := struct {
		env                 string
		appName             string
		loggingFormat       string
		sentryDsn           string
		fakeAwsS3Repository bool
	}{
		env:                 utils.GetEnv("ENV", "development"),
		appName:             "marble-backend",
		loggingFormat:       utils.GetEnv("LOGGING_FORMAT", "text"),
		sentryDsn:           utils.GetEnv("SENTRY_DSN", ""),
		fakeAwsS3Repository: utils.GetEnv("FAKE_AWS_S3", false),
	}

	logger := utils.NewLogger(jobConfig.loggingFormat)

	infra.SetupSentry(jobConfig.sentryDsn, jobConfig.env)
	defer sentry.Flush(3 * time.Second)

	tracingConfig := infra.TelemetryConfiguration{
		ApplicationName: jobConfig.appName,
		Enabled:         gcpConfig.EnableTracing,
		ProjectID:       gcpConfig.ProjectId,
	}
	_, err := infra.InitTelemetry(tracingConfig)
	if err != nil {
		logger.ErrorContext(ctx, "failed to initialize tracing", slog.String("error", err.Error()))
		return err
	}

	pool, err := infra.NewPostgresConnectionPool(ctx, pgConfig.GetConnectionString())
	if err != nil {
		logger.ErrorContext(ctx, "failed to create marbleConnectionPool", slog.String("error", err.Error()))
		return err
	}

	repositories := repositories.NewRepositories(nil, pool, nil, "")

	uc := usecases.NewUsecases(repositories,
		usecases.WithFakeAwsS3Repository(jobConfig.fakeAwsS3Repository))

	err = jobs.ExecuteAllScheduledScenarios(ctx, uc, tracingConfig)
	if err != nil {
		logger.ErrorContext(ctx, "failed to execute all scheduled scenarios", slog.String("error", err.Error()))
	}

	return err
}
