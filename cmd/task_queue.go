package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
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
		// GoogleApplicationCredentials: utils.GetEnv("GOOGLE_APPLICATION_CREDENTIALS", ""),
	}
	pgConfig := infra.PgConfig{
		Database:            "marble",
		DbConnectWithSocket: utils.GetEnv("PG_CONNECT_WITH_SOCKET", false),
		Hostname:            utils.GetRequiredEnv[string]("PG_HOSTNAME"),
		Password:            utils.GetRequiredEnv[string]("PG_PASSWORD"),
		Port:                utils.GetEnv("PG_PORT", "5432"),
		User:                utils.GetRequiredEnv[string]("PG_USER"),
	}

	// licenseConfig := models.LicenseConfiguration{
	// 	LicenseKey:             utils.GetEnv("LICENSE_KEY", ""),
	// 	KillIfReadLicenseError: utils.GetEnv("KILL_IF_READ_LICENSE_ERROR", false),
	// }
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
	// license := infra.VerifyLicense(licenseConfig)

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

	pool, err := infra.NewPostgresConnectionPool(ctx, pgConfig.GetConnectionString(), telemetryRessources.TracerProvider)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, &models.SortWorker{})

	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Workers: workers,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {
				MaxWorkers: 3,
			},
		},
	})
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return err
	}

	// repositories := repositories.NewRepositories(
	// 	pool,
	// 	gcpConfig.GoogleApplicationCredentials,
	// )

	// uc := usecases.NewUsecases(repositories,
	// 	usecases.WithLicense(license),
	// 	usecases.WithRiverClient(riverClient),
	// )

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
		fmt.Printf("Received SIGINT/SIGTERM; initiating soft stop (try to wait for jobs to finish)\n")

		softStopCtx, softStopCtxCancel := context.WithTimeout(ctx, 10*time.Second)
		defer softStopCtxCancel()

		go func() {
			select {
			case <-sigintOrTerm:
				fmt.Printf("Received SIGINT/SIGTERM again; initiating hard stop (cancel everything)\n")
				softStopCtxCancel()
			case <-softStopCtx.Done():
				fmt.Printf("Soft stop timeout; initiating hard stop (cancel everything)\n")
			}
		}()

		err := riverClient.Stop(softStopCtx)
		if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			panic(err)
		}
		if err == nil {
			fmt.Printf("Soft stop succeeded\n")
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
			fmt.Printf("Hard stop timeout; ignoring stop procedure and exiting unsafely\n")
		} else if err != nil {
			panic(err)
		}

		// hard stop succeeded
	}()

	// Make sure our job starts being worked before doing anything else.
	// <-jobStarted

	// Cheat a little by sending a SIGTERM manually for the purpose of this
	// example (normally this will be sent by user or supervisory process). The
	// first SIGTERM tries a soft stop in which jobs are given a chance to
	// finish up.
	// sigintOrTerm <- syscall.SIGTERM

	// // The soft stop will never work in this example because our job only
	// // respects context cancellation, but wait a short amount of time to give it
	// // a chance. After it elapses, send another SIGTERM to initiate a hard stop.
	select {
	case <-riverClient.Stopped():
		// Will never be reached in this example because our job will only ever
		// finish on context cancellation.
		fmt.Printf("Soft stop succeeded\n")

	case <-time.After(4 * time.Second):
		sigintOrTerm <- syscall.SIGTERM
		<-riverClient.Stopped()
	}

	return nil
}
