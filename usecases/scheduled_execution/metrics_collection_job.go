package scheduled_execution

import (
	"context"
	"slices"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/metrics_collection"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// const METRIC_COLLECTION_WORKER_INTERVAL = 24 * time.Hour // Run daily
const METRICS_COLLECTION_WORKER_INTERVAL = 10 * time.Second // Run every minute for testing

func NewMetricsCollectionPeriodicJob() *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(METRICS_COLLECTION_WORKER_INTERVAL),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.MetricsCollectionArgs{}, &river.InsertOpts{
				Queue: "global",
				UniqueOpts: river.UniqueOpts{
					ByQueue:  true,
					ByPeriod: METRICS_COLLECTION_WORKER_INTERVAL,
				},
			}
		},
		&river.PeriodicJobOpts{RunOnStart: false},
	)
}

type MetricCollectionWorker struct {
	river.WorkerDefaults[models.MetricsCollectionArgs]

	executorFactory        executor_factory.ExecutorFactory
	organizationRepository repositories.OrganizationRepository
	collectors             metrics_collection.Collectors
}

func NewMetricCollectionWorker(
	executorFactory executor_factory.ExecutorFactory,
	organizationRepository repositories.OrganizationRepository,
	collectors metrics_collection.Collectors,
) MetricCollectionWorker {
	return MetricCollectionWorker{
		executorFactory:        executorFactory,
		organizationRepository: organizationRepository,
		collectors:             collectors,
	}
}

// Work executes the metrics collection job by collecting both global and organization-specific metrics
// from a time range defined by watermarks, then updates the watermark to track progress
func (w MetricCollectionWorker) Work(ctx context.Context, job *river.Job[models.MetricsCollectionArgs]) error {
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Starting metrics collection job")

	now := time.Now()

	// Take a watermark for the "from" time
	// TODO: Get the from time from watermark or a default value if not exists (-> create a function for this)
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create the metric collection usecase
	metrics, err := w.collectMetrics(ctx, from, now)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to collect metrics", "error", err)
		return err
	}

	logger.DebugContext(ctx, "Collected metrics", "metrics", metrics)

	// TODO: Update the watermarks with now value
	logger.DebugContext(ctx, "Updating watermarks", "new_timestamp", now)

	// TODO: Store or send the metrics somewhere
	// For now, just log the number of metrics collected
	logger.DebugContext(ctx, "Metric collection completed", "metrics_count", len(metrics.Metrics))

	return nil
}

func (w *MetricCollectionWorker) collectMetrics(ctx context.Context, from time.Time, to time.Time) (models.MetricsPayload, error) {
	metrics := make([]models.MetricData, 0)

	// Collect global metrics
	globalMetrics, err := w.collectGlobalMetrics(ctx, from, to)
	if err != nil {
		return models.MetricsPayload{}, err
	}
	metrics = slices.Concat(metrics, globalMetrics)

	// Collect organization-specific metrics
	orgMetrics, err := w.collectOrganizationMetrics(ctx, from, to)
	if err != nil {
		return models.MetricsPayload{}, err
	}
	metrics = slices.Concat(metrics, orgMetrics)

	payload := models.MetricsPayload{
		CollectionID: uuid.New(),
		Timestamp:    time.Now(),
		Metrics:      metrics,
		Version:      w.collectors.GetVersion(),
	}

	return payload, nil
}

// Collects global metrics from all collectors
// If a collector fails, it will log a warning and continue to the next collector (don't fail the whole function)
func (w *MetricCollectionWorker) collectGlobalMetrics(ctx context.Context, from time.Time, to time.Time) ([]models.MetricData, error) {
	metrics := make([]models.MetricData, 0)
	logger := utils.LoggerFromContext(ctx)

	for _, collector := range w.collectors.GetGlobalCollectors() {
		value, err := collector.Collect(ctx, from, to)
		if err != nil {
			logger.WarnContext(ctx, "Failed to collect global metrics", "error", err)
			continue
		}
		metrics = slices.Concat(metrics, value)
	}

	return metrics, nil
}

// Collects organization metrics from all collectors, fetching all organizations from the database first
// If a collector fails, it will log a warning and continue to the next collector (don't fail the whole function)
func (w *MetricCollectionWorker) collectOrganizationMetrics(ctx context.Context, from time.Time, to time.Time) ([]models.MetricData, error) {
	metrics := make([]models.MetricData, 0)
	logger := utils.LoggerFromContext(ctx)

	orgs, err := w.getListOfOrganizations(ctx)
	if err != nil {
		return []models.MetricData{}, err
	}

	for _, org := range orgs {
		for _, collector := range w.collectors.GetCollectors() {
			value, err := collector.Collect(ctx, org.Id, from, to)
			if err != nil {
				logger.WarnContext(ctx, "Failed to collect organization metrics", "error", err)
				continue
			}
			metrics = slices.Concat(metrics, value)
		}
	}

	return metrics, nil
}

// Fetches all organizations from the database
// NOTE: Add caching to avoid fetching the same organizations every time (but how can we invalidate the cache?)
func (w *MetricCollectionWorker) getListOfOrganizations(ctx context.Context) ([]models.Organization, error) {
	orgs, err := w.organizationRepository.AllOrganizations(ctx, w.executorFactory.NewExecutor())
	if err != nil {
		return []models.Organization{}, err
	}
	return orgs, nil
}
