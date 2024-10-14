package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/jobs"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/getsentry/sentry-go"
)

func RunTaskQueue() error {
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
	serverConfig := struct {
		env           string
		loggingFormat string
		sentryDsn     string
	}{
		env:           utils.GetEnv("ENV", "development"),
		loggingFormat: utils.GetEnv("LOGGING_FORMAT", "text"),
		sentryDsn:     utils.GetEnv("SENTRY_DSN", ""),
	}

	logger := utils.NewLogger(serverConfig.loggingFormat)
	ctx := utils.StoreLoggerInContext(context.Background(), logger)
	license := infra.VerifyLicense(licenseConfig)

	infra.SetupSentry(serverConfig.sentryDsn, serverConfig.env)
	defer sentry.Flush(3 * time.Second)

	tracingConfig := infra.TelemetryConfiguration{
		ApplicationName: "marble",
		Enabled:         gcpConfig.EnableTracing,
		ProjectID:       gcpConfig.ProjectId,
	}
	telemetryRessources, err := infra.InitTelemetry(tracingConfig)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
	}
	ctx = utils.StoreOpenTelemetryTracerInContext(ctx, telemetryRessources.Tracer)

	pool, err := infra.NewPostgresConnectionPool(ctx, pgConfig.GetConnectionString(), telemetryRessources.TracerProvider)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
	}

	workers := river.NewWorkers()
	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Workers: workers,
		Queues: map[string]river.QueueConfig{
			repositories.DecisionsQueueName("54624b1f-aac3-4d3c-8fee-75db36436e12"): { // TODO: remove hard coded org id, get list of org ids from query instead. + Add periodic refresh with "add queue"
				MaxWorkers: 3,
			},
		},
	})
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return err
	}

	repositories := repositories.NewRepositories(
		pool,
		gcpConfig.GoogleApplicationCredentials,
		repositories.WithRiverClient(riverClient),
		repositories.WithConvoyClientProvider(
			infra.InitializeConvoyRessources(convoyConfiguration),
			convoyConfiguration.RateLimit,
		),
	)

	uc := usecases.NewUsecases(repositories,
		usecases.WithLicense(license),
	)

	adminUc := jobs.GenerateUsecaseWithCredForMarbleAdmin(ctx, uc)
	river.AddWorker(workers, adminUc.NewAsyncDecisionWorker())
	river.AddWorker(workers, adminUc.NewNewAsyncScheduledExecWorker())

	if err := riverClient.Start(ctx); err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return err
	}

	sigintOrTerm := make(chan os.Signal, 1)
	signal.Notify(sigintOrTerm, syscall.SIGINT, syscall.SIGTERM)

	// This is meant to be a realistic-looking stop goroutine that might go in a
	// real program. It waits for SIGINT/SIGTERM and when received, tries to stop
	// gracefully by allowing a chance for jobs to finish. But if that isn't
	// working, a second SIGINT/SIGTERM will tell it to terminate with prejudice and
	// it'll issue a hard stop that cancels the context of all active jobs. In
	// case that doesn't work, a third SIGINT/SIGTERM ignores River's stop procedure
	// completely and exits uncleanly.
	go func() {
		<-sigintOrTerm
		logger.InfoContext(ctx, "Received SIGINT/SIGTERM; initiating soft stop (try to wait for jobs to finish)")

		softStopCtx, softStopCtxCancel := context.WithTimeout(ctx, 5*time.Second)
		defer softStopCtxCancel()

		go func() {
			select {
			case <-sigintOrTerm:
				logger.InfoContext(ctx, "Received SIGINT/SIGTERM again; initiating hard stop (cancel everything)")
				softStopCtxCancel()
			case <-softStopCtx.Done():
				logger.InfoContext(ctx, "Soft stop timeout; initiating hard stop (cancel everything)")
			}
		}()

		err := riverClient.Stop(softStopCtx)
		if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			logger.ErrorContext(ctx, "Soft stop failed", "error", err)
			panic(err)
		}
		if err == nil {
			logger.InfoContext(ctx, "Soft stop succeeded")
			return
		}

		hardStopCtx, hardStopCtxCancel := context.WithTimeout(ctx, 10*time.Second)
		defer hardStopCtxCancel()

		// As long as all jobs respect context cancellation, StopAndCancel will
		// always work. However, in the case of a bug where a job blocks despite
		// being cancelled, it may be necessary to either ignore River's stop
		// result (what's shown here) or have a supervisor kill the process.
		err = riverClient.StopAndCancel(hardStopCtx)
		if err != nil && errors.Is(err, context.DeadlineExceeded) {
			logger.InfoContext(ctx, "Hard stop timeout; ignoring stop procedure and exiting unsafely")
		} else if err != nil {
			panic(err)
		}
		// hard stop succeeded
	}()

	// The soft stop will never work in this example because our job only
	// respects context cancellation, but wait a short amount of time to give it
	// a chance. After it elapses, send another SIGTERM to initiate a hard stop.
	logger.InfoContext(ctx, "Waiting for SIGINT/SIGTERM to stop")
	select {
	case <-riverClient.Stopped():
		logger.InfoContext(ctx, "Soft stop succeeded")
		return nil
	case <-time.After(300 * time.Second): // TODO remove this eventually, it's for testing purposes
		logger.InfoContext(ctx, "Sigterm now")
		sigintOrTerm <- syscall.SIGTERM
		<-riverClient.Stopped()
	}

	<-riverClient.Stopped()
	logger.InfoContext(ctx, "Soft stop succeeded")

	return nil
}
