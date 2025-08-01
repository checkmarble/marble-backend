package cmd

import (
	"context"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/jobs"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/scheduled_execution"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/cockroachdb/errors"
	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
)

func RunTaskQueue(apiVersion string) error {
	// This is where we read the environment variables and set up the configuration for the application.
	gcpConfig, err := infra.NewGcpConfig(
		context.Background(),
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

	licenseConfig := models.LicenseConfiguration{
		LicenseKey:             utils.GetEnv("LICENSE_KEY", ""),
		KillIfReadLicenseError: utils.GetEnv("KILL_IF_READ_LICENSE_ERROR", false),
	}

	workerConfig := struct {
		appName                     string
		env                         string
		failedWebhooksRetryPageSize int
		ingestionBucketUrl          string
		loggingFormat               string
		sentryDsn                   string
		cloudRunProbePort           string
		caseReviewTimeout           time.Duration
		caseManagerBucket           string
	}{
		appName:                     "marble-backend",
		env:                         utils.GetEnv("ENV", "development"),
		failedWebhooksRetryPageSize: utils.GetEnv("FAILED_WEBHOOKS_RETRY_PAGE_SIZE", 1000),
		ingestionBucketUrl:          utils.GetRequiredEnv[string]("INGESTION_BUCKET_URL"),
		loggingFormat:               utils.GetEnv("LOGGING_FORMAT", "text"),
		sentryDsn:                   utils.GetEnv("SENTRY_DSN", ""),
		cloudRunProbePort:           utils.GetEnv("CLOUD_RUN_PROBE_PORT", ""),
		caseReviewTimeout:           utils.GetEnvDuration("AI_CASE_REVIEW_TIMEOUT", 5*time.Minute),
		caseManagerBucket:           utils.GetEnv("CASE_MANAGER_BUCKET_URL", ""),
	}

	logger := utils.NewLogger(workerConfig.loggingFormat)
	ctx := utils.StoreLoggerInContext(context.Background(), logger)
	license := infra.VerifyLicense(licenseConfig)

	offloadingConfig := infra.OffloadingConfig{
		Enabled:         utils.GetEnv("OFFLOADING_ENABLED", false),
		BucketUrl:       utils.GetEnv("OFFLOADING_BUCKET_URL", ""),
		JobInterval:     utils.GetEnvDuration("OFFLOADING_JOB_INTERVAL", 30*time.Minute),
		OffloadBefore:   utils.GetEnvDuration("OFFLOADING_BEFORE", 7*24*time.Hour),
		BatchSize:       utils.GetEnv("OFFLOADING_BATCH_SIZE", 10_000),
		SavepointEvery:  utils.GetEnv("OFFLOADING_SAVE_POINTS", 100),
		WritesPerSecond: utils.GetEnv("OFFLOADING_WRITES_PER_SEC", 200),
	}

	offloadingConfig.ValidateAndFix(ctx)

	metricCollectionConfig := infra.MetricCollectionConfig{
		Disabled:         utils.GetEnv("DISABLE_TELEMETRY", false),
		JobInterval:      utils.GetEnvDuration("METRICS_COLLECTION_JOB_INTERVAL", 1*time.Hour),
		FallbackDuration: utils.GetEnvDuration("METRICS_FALLBACK_DURATION", 30*24*time.Hour),
	}
	metricCollectionConfig.Configure(licenseConfig)

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
	}

	infra.SetupSentry(workerConfig.sentryDsn, workerConfig.env, apiVersion)
	defer sentry.Flush(3 * time.Second)

	tracingConfig := infra.TelemetryConfiguration{
		ApplicationName: workerConfig.appName,
		Enabled:         gcpConfig.EnableTracing,
		ProjectID:       gcpConfig.ProjectId,
	}
	telemetryRessources, err := infra.InitTelemetry(tracingConfig, apiVersion)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
	}
	ctx = utils.StoreOpenTelemetryTracerInContext(ctx, telemetryRessources.Tracer)

	pool, err := infra.NewPostgresConnectionPool(ctx, pgConfig.GetConnectionString(),
		telemetryRessources.TracerProvider, pgConfig.MaxPoolConnections)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
	}

	// First, create an insert-only client to pass to the repos. Later we create another client with a list of queues (org_ids)
	// but we need working repos first. It's a bit awkward but it's a consequence of the fact that river uses the same client for
	// job insertion and job running.
	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return err
	}

	clientDbConfig, err := infra.ParseClientDbConfig(pgConfig.ClientDbConfigFile)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return err
	}

	repositories := repositories.NewRepositories(
		pool,
		infra.GcpConfig{},
		repositories.WithRiverClient(riverClient),
		repositories.WithConvoyClientProvider(
			infra.InitializeConvoyRessources(convoyConfiguration),
			convoyConfiguration.RateLimit,
		),
		repositories.WithClientDbConfig(clientDbConfig),
		repositories.WithTracerProvider(telemetryRessources.TracerProvider),
		repositories.WithOpenSanctions(openSanctionsConfig),
	)

	// Start the task queue workers
	workers := river.NewWorkers()
	queues, orgPeriodics, err := usecases.QueuesFromOrgs(ctx, &repositories.MarbleDbRepository,
		repositories.ExecutorGetter, offloadingConfig)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return err
	}

	// For non-org
	nonOrgQueues := make(map[string]river.QueueConfig)
	globalPeriodics := []*river.PeriodicJob{}

	if !metricCollectionConfig.Disabled {
		metricQueue := usecases.QueueMetrics()
		maps.Copy(nonOrgQueues, metricQueue)
		globalPeriodics = append(globalPeriodics,
			scheduled_execution.NewMetricsCollectionPeriodicJob(metricCollectionConfig))
	}

	maps.Copy(queues, nonOrgQueues)

	// Periodics always contain the per-org tasks retrieved above. Add other, non-organization-scoped periodics below
	periodics := append(
		orgPeriodics,
		globalPeriodics...,
	)

	riverClient, err = river.NewClient(riverpgxv5.New(pool), &river.Config{
		FetchPollInterval: 100 * time.Millisecond,
		Queues:            queues,

		// Must be larger than the time it takes to process a job. Increase it if we want to use longer-lived jobs.
		RescueStuckJobsAfter: 1 * time.Minute,
		WorkerMiddleware: []rivertype.WorkerMiddleware{
			jobs.NewTracingMiddleware(telemetryRessources.Tracer),
			jobs.NewSentryMiddleware(),
			jobs.NewLoggerMiddleware(logger),
			jobs.NewRecoveredMiddleware(),
		},
		Workers:      workers,
		PeriodicJobs: periodics,
	},
	)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return err
	}

	// Ensure that all global queues are active.
	if err := ensureGlobalQueuesAreActive(ctx, riverClient, nonOrgQueues); err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return err
	}

	uc := usecases.NewUsecases(repositories,
		usecases.WithIngestionBucketUrl(workerConfig.ingestionBucketUrl),
		usecases.WithOffloading(offloadingConfig),
		usecases.WithFailedWebhooksRetryPageSize(workerConfig.failedWebhooksRetryPageSize),
		usecases.WithLicense(license),
		usecases.WithConvoyServer(convoyConfiguration.APIUrl),
		usecases.WithOpensanctions(openSanctionsConfig.IsSet()),
		usecases.WithApiVersion(apiVersion),
		usecases.WithMetricsCollectionConfig(metricCollectionConfig),
		usecases.WithCaseManagerBucketUrl(workerConfig.caseManagerBucket),
		usecases.WithAIAgentConfig(aiAgentConfig),
	)
	adminUc := jobs.GenerateUsecaseWithCredForMarbleAdmin(ctx, uc)
	river.AddWorker(workers, adminUc.NewAsyncDecisionWorker())
	river.AddWorker(workers, adminUc.NewNewAsyncScheduledExecWorker())
	river.AddWorker(workers, adminUc.NewIndexCreationWorker())
	river.AddWorker(workers, adminUc.NewIndexCreationStatusWorker())
	river.AddWorker(workers, adminUc.NewIndexCleanupWorker())
	river.AddWorker(workers, adminUc.NewIndexDeletionWorker())
	river.AddWorker(workers, adminUc.NewTestRunSummaryWorker())
	river.AddWorker(workers, adminUc.NewMatchEnrichmentWorker())
	river.AddWorker(workers, adminUc.NewCaseReviewWorker(workerConfig.caseReviewTimeout))
	river.AddWorker(workers, adminUc.NewAutoAssignmentWorker())

	if offloadingConfig.Enabled {
		river.AddWorker(workers, adminUc.NewOffloadingWorker())
	}
	if !metricCollectionConfig.Disabled {
		river.AddWorker(workers, uc.NewMetricsCollectionWorker(licenseConfig))
	}

	if err := riverClient.Start(ctx); err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return err
	}

	// run a non-blocking basic http server to respond to Cloud Run http probes, to respect the Cloud Run contract
	if workerConfig.cloudRunProbePort != "" {
		go func() {
			http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			})
			if err := http.ListenAndServe(":"+workerConfig.cloudRunProbePort, nil); err != nil {
				utils.LogAndReportSentryError(ctx, err)
			}
		}()
	}

	logger.InfoContext(ctx, "starting worker", slog.String("version", apiVersion))

	// Asynchronously keep the task queue workers up to date with the orgs in the database
	taskQueueWorker := uc.NewTaskQueueWorker(riverClient,
		slices.Collect(maps.Keys(nonOrgQueues)),
	)
	go taskQueueWorker.RefreshQueuesFromOrgIds(ctx)

	// Start the cron jobs using the old entrypoint.
	// This will progressively be replaced by the new task queue system.
	// We do not wait for it, the state of the job is handled by the task queue workers.
	go jobs.RunScheduler(ctx, uc)

	// Teardown sequence
	sigintOrTerm := make(chan os.Signal, 1)
	signal.Notify(sigintOrTerm, syscall.SIGINT, syscall.SIGTERM)

	go cleanStop(ctx, sigintOrTerm, riverClient)

	<-riverClient.Stopped()
	logger.InfoContext(ctx, "River client stopped")

	return nil
}

// This stop goroutine waits for SIGINT/SIGTERM and when received, tries to stop
// gracefully by allowing a chance for jobs to finish. But if that isn't
// working, a second SIGINT/SIGTERM will tell it to terminate with prejudice and
// it'll issue a hard stop that cancels the context of all active jobs. In
// case that doesn't work, a third SIGINT/SIGTERM ignores River's stop procedure
// completely and exits uncleanly.
func cleanStop(ctx context.Context, sigintOrTerm chan os.Signal, riverClient *river.Client[pgx.Tx]) {
	logger := utils.LoggerFromContext(ctx)
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
}

// Ensure that all global queues are active in river DB, i.e. not paused. This function is more a safety net.
// River stores the state of the queues in the DB. If a queue exists but is paused, River client will not resume it
// We have some function which can pause queues, so we need to ensure that all global queues are active
// cf: riverClient.QueueResume and riverClient.QueuePause
func ensureGlobalQueuesAreActive(ctx context.Context, riverClient *river.Client[pgx.Tx],
	nonOrgQueues map[string]river.QueueConfig,
) error {
	logger := utils.LoggerFromContext(ctx)

	for queueName := range nonOrgQueues {
		queueState, err := riverClient.QueueGet(ctx, queueName)
		if err != nil {
			if errors.Is(err, river.ErrNotFound) {
				// Queue will be created when River starts, skip
				continue
			}
			return err
		}

		// If the queue exists and is paused, resume it
		if queueState.PausedAt != nil {
			logger.InfoContext(ctx, "Resuming global queue at startup", "queue", queueName)
			if err := riverClient.QueueResume(ctx, queueName, &river.QueuePauseOpts{}); err != nil {
				return err
			}
		}
	}

	return nil
}
