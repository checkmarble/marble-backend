package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"reflect"
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
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/cockroachdb/errors"
	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
)

func RunTaskQueue(apiVersion string, only, onlyArgs string) error {
	appName := fmt.Sprintf("marble-worker %s", apiVersion)

	// This is where we read the environment variables and set up the configuration for the application.
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
	if pgConfig.ConnectionString != "" {
		if u, err := url.Parse(pgConfig.ConnectionString); err != nil || !u.IsAbs() {
			switch err {
			case nil:
				return errors.New("invalid database connection string")
			default:
				return errors.Wrap(err, "invalid database connection string")
			}
		}
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

	workerConfig := WorkerConfig{
		appName:                     "marble-backend-worker",
		env:                         utils.GetEnv("ENV", "production"),
		failedWebhooksRetryPageSize: utils.GetEnv("FAILED_WEBHOOKS_RETRY_PAGE_SIZE", 1000),
		ingestionBucketUrl:          utils.GetRequiredEnv[string]("INGESTION_BUCKET_URL"),
		loggingFormat:               utils.GetEnv("LOGGING_FORMAT", "text"),
		sentryDsn:                   utils.GetEnv("SENTRY_DSN", ""),
		cloudRunProbePort:           utils.GetEnv("CLOUD_RUN_PROBE_PORT", ""),
		caseReviewTimeout:           utils.GetEnvDuration("AI_CASE_REVIEW_TIMEOUT", 5*time.Minute),
		caseManagerBucket:           utils.GetEnv("CASE_MANAGER_BUCKET_URL", ""),
		analyticsBucket:             utils.GetEnv("ANALYTICS_BUCKET_URL", ""),
		telemetryExporter:           utils.GetEnv("TRACING_EXPORTER", "otlp"),
		otelSamplingRates:           utils.GetEnv("TRACING_SAMPLING_RATES", ""),
		enablePrometheus:            utils.GetEnv("ENABLE_PROMETHEUS", false),
		enableTracing:               utils.GetEnv("ENABLE_TRACING", false),
	}

	logger := utils.NewLogger(workerConfig.loggingFormat)
	ctx := utils.StoreLoggerInContext(context.Background(), logger)

	gcpConfig, err := infra.NewGcpConfig(
		ctx,
		utils.GetEnv("GOOGLE_CLOUD_PROJECT", ""),
		utils.GetEnv("GOOGLE_APPLICATION_CREDENTIALS", ""),
	)
	if err != nil {
		logger.WarnContext(ctx, "could not initialize GCP config", "error", err.Error())
	}
	isMarbleSaasProject := infra.IsMarbleSaasProject()

	offloadingConfig := infra.OffloadingConfig{
		Enabled:         utils.GetEnv("OFFLOADING_ENABLED", false),
		BucketUrl:       utils.GetEnv("OFFLOADING_BUCKET_URL", ""),
		JobInterval:     utils.GetEnvDuration("OFFLOADING_JOB_INTERVAL", 30*time.Minute),
		OffloadBefore:   utils.GetEnvDuration("OFFLOADING_BEFORE", 7*24*time.Hour),
		BatchSize:       utils.GetEnv("OFFLOADING_BATCH_SIZE", 1000),
		SavepointEvery:  utils.GetEnv("OFFLOADING_SAVE_POINTS", 100),
		WritesPerSecond: utils.GetEnv("OFFLOADING_WRITES_PER_SEC", 200),
	}

	var analyticsConfig infra.AnalyticsConfig

	if analyticsConfig, err = infra.InitAnalyticsConfig(pgConfig, workerConfig.analyticsBucket); err != nil {
		return err
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
		PerplexityAPIKey:  utils.GetEnv("AI_AGENT_PERPLEXITY_API_KEY", ""),
	}

	infra.SetupSentry(workerConfig.sentryDsn, workerConfig.env, apiVersion)
	defer sentry.Flush(3 * time.Second)

	tracingConfig := infra.TelemetryConfiguration{
		ApplicationName: workerConfig.appName,
		Enabled:         workerConfig.enableTracing,
		ProjectID:       gcpConfig.ProjectId,
		Exporter:        workerConfig.telemetryExporter,
		SamplingMap:     infra.NewTelemetrySamplingMap(ctx, workerConfig.otelSamplingRates),
	}
	telemetryRessources, err := infra.InitTelemetry(tracingConfig, apiVersion)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
	}
	ctx = utils.StoreOpenTelemetryTracerInContext(ctx, telemetryRessources.Tracer)

	pool, err := infra.NewPostgresConnectionPool(ctx, appName, pgConfig.GetConnectionString(),
		telemetryRessources.TracerProvider, pgConfig.MaxPoolConnections, pgConfig.ImpersonateRole)
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

	var lagoConfig infra.LagoConfig
	if isMarbleSaasProject {
		lagoConfig = infra.InitializeLago()
		if err := lagoConfig.Validate(); err != nil {
			utils.LogAndReportSentryError(ctx, err)
		}
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
		repositories.WithCache(utils.GetEnv("CACHE_ENABLED", false)),
		repositories.WithLagoConfig(lagoConfig),
	)

	deploymentMetadata, err := GetDeploymentMetadata(ctx, repositories)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return errors.Wrap(err, "failed to get deployment ID from Marble DB")
	}
	license := infra.VerifyLicense(licenseConfig, deploymentMetadata.Value)

	// Start the task queue workers
	workers := river.NewWorkers()
	queues, orgPeriodics, err := usecases.QueuesFromOrgs(ctx, appName, repositories.MarbleDbRepository,
		repositories.ExecutorGetter, offloadingConfig, analyticsConfig)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return err
	}

	// For non-org
	nonOrgQueues := make(map[string]river.QueueConfig)
	globalPeriodics := []*river.PeriodicJob{}

	if !metricCollectionConfig.Disabled {
		maps.Copy(nonOrgQueues, usecases.QueueMetrics())
		globalPeriodics = append(globalPeriodics,
			scheduled_execution.NewMetricsCollectionPeriodicJob(metricCollectionConfig))
	}
	if analyticsConfig.Enabled {
		analyticsQueue := usecases.QueueAnalyticsMerge()
		maps.Copy(nonOrgQueues, analyticsQueue)
		globalPeriodics = append(globalPeriodics,
			scheduled_execution.NewAnalyticsMergeJob())
	}
	if isMarbleSaasProject && lagoConfig.IsConfigured() {
		maps.Copy(nonOrgQueues, usecases.QueueBilling())
	}
	// Merge non-org queues with org queues
	maps.Copy(queues, nonOrgQueues)

	// Periodics always contain the per-org tasks retrieved above. Add other, non-organization-scoped periodics below
	periodics := append(
		orgPeriodics,
		globalPeriodics...,
	)

	riverClient, err = river.NewClient(riverpgxv5.New(pool), &river.Config{
		FetchPollInterval: 100 * time.Millisecond,
		Queues:            queues,

		// Must be larger than the time it takes to process a job, if the job does not implement Timeout().
		// Jobs that do not implement this and run for longer than this value will be rescued by the worker, which we should
		// avoid if it is actually still running.
		RescueStuckJobsAfter: 2 * time.Minute,
		WorkerMiddleware: []rivertype.WorkerMiddleware{
			jobs.NewRecoveredMiddleware(),
			jobs.NewSentryMiddleware(),
			jobs.NewTracingMiddleware(telemetryRessources.Tracer),
			jobs.NewLoggerMiddleware(logger),
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
		usecases.WithAppName(appName),
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
		usecases.WithAnalyticsConfig(analyticsConfig),
	)
	adminUc := jobs.GenerateUsecaseWithCredForMarbleAdmin(ctx, uc)

	if only != "" {
		if err := singleJobRun(ctx, adminUc, only, onlyArgs); err != nil {
			logger.Error(err.Error())
		}

		return nil
	}

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
	river.AddWorker(workers, adminUc.NewDecisionWorkflowsWorker())
	if offloadingConfig.Enabled {
		river.AddWorker(workers, adminUc.NewOffloadingWorker())
	}
	if !metricCollectionConfig.Disabled {
		river.AddWorker(workers, uc.NewMetricsCollectionWorker(licenseConfig))
	}
	if analyticsConfig.Enabled {
		river.AddWorker(workers, adminUc.NewAnalyticsExportWorker())
		river.AddWorker(workers, adminUc.NewAnalyticsMergeWorker())
	}
	if isMarbleSaasProject && lagoConfig.IsConfigured() {
		river.AddWorker(workers, uc.NewSendBillingEventWorker())
	}

	if err := riverClient.Start(ctx); err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return err
	}

	// run a non-blocking basic http server to respond to Cloud Run http probes, to respect the Cloud Run contract
	if workerConfig.cloudRunProbePort != "" {
		go func() {
			gin.SetMode(gin.ReleaseMode)

			r := gin.New()

			r.GET("/", func(c *gin.Context) {
				c.String(http.StatusOK, "OK")
			})

			if workerConfig.enablePrometheus {
				r.GET("/metrics", gin.WrapH(promhttp.Handler()))
			}

			if os.Getenv("DEBUG_ENABLE_PROFILING") == "1" {
				utils.SetupProfilerEndpoints(r, "marble-worker", apiVersion, gcpConfig.ProjectId)
			}

			if err := r.Run(":" + workerConfig.cloudRunProbePort); err != nil {
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

func singleJobRun(ctx context.Context, uc usecases.UsecasesWithCreds, jobName, jobArgs string) error {
	switch jobName {
	case "async_decision":
		return uc.NewAsyncDecisionWorker().Work(ctx,
			singleJobCreate[models.AsyncDecisionArgs](ctx, jobArgs))
	case "scheduled_exec_status":
		return uc.NewNewAsyncScheduledExecWorker().Work(ctx,
			singleJobCreate[models.ScheduledExecStatusSyncArgs](ctx, jobArgs))
	case "auto_assignment":
		return uc.NewAutoAssignmentWorker().Work(ctx,
			singleJobCreate[models.AutoAssignmentArgs](ctx, jobArgs))
	case "index_cleanup":
		return uc.NewIndexCleanupWorker().Work(ctx,
			singleJobCreate[models.IndexCleanupArgs](ctx, jobArgs))
	case "index_creation":
		return uc.NewIndexCreationWorker().Work(ctx,
			singleJobCreate[models.IndexCreationArgs](ctx, jobArgs))
	case "index_creation_status":
		return uc.NewIndexCreationStatusWorker().Work(ctx,
			singleJobCreate[models.IndexCreationStatusArgs](ctx, jobArgs))
	case "index_deletion":
		return uc.NewIndexDeletionWorker().Work(ctx,
			singleJobCreate[models.IndexDeletionArgs](ctx, jobArgs))
	case "match_enrichment":
		return uc.NewMatchEnrichmentWorker().Work(ctx,
			singleJobCreate[models.MatchEnrichmentArgs](ctx, jobArgs))
	case "offloading":
		return uc.NewOffloadingWorker().Work(ctx,
			singleJobCreate[models.OffloadingArgs](ctx, jobArgs))
	case "test_run_summary":
		return uc.NewTestRunSummaryWorker().Work(ctx,
			singleJobCreate[models.TestRunSummaryArgs](ctx, jobArgs))
	case "case_review":
		return uc.NewCaseReviewWorker(time.Hour).Work(ctx,
			singleJobCreate[models.CaseReviewArgs](ctx, jobArgs))
	case "decision_workflows":
		return uc.NewDecisionWorkflowsWorker().Work(ctx,
			singleJobCreate[models.DecisionWorkflowArgs](ctx, jobArgs))
	case "analytics_export":
		return uc.NewAnalyticsExportWorker().Work(ctx,
			singleJobCreate[models.AnalyticsExportArgs](ctx, jobArgs))
	case "analytics_merge":
		return uc.NewAnalyticsMergeWorker().Work(ctx,
			singleJobCreate[models.AnalyticsMergeArgs](ctx, jobArgs))
	case "send_billing_event":
		return uc.NewSendBillingEventWorker().Work(ctx,
			singleJobCreate[models.SendBillingEventArgs](ctx, jobArgs))
	default:
		return errors.Newf("unknown job %s", jobName)
	}
}

func singleJobCreate[A river.JobArgs](ctx context.Context, argsJson string) *river.Job[A] {
	var args A

	if argsJson != "" {
		if err := json.Unmarshal([]byte(argsJson), &args); err != nil {
			utils.LoggerFromContext(ctx).Error("could not unmarshal provided JSON into job arguments", "error", err.Error())
			os.Exit(1)
		}
	}

	if reflect.DeepEqual(args, *new(A)) {
		utils.LoggerFromContext(ctx).Warn("job arguments unmarshalled to the zero struct, job might not run properly")
	}

	return &river.Job[A]{
		JobRow: &rivertype.JobRow{CreatedAt: time.Now()},
		Args:   args,
	}
}
