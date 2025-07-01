package scheduled_execution

import (
	"context"
	"maps"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/metrics_collection"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/riverqueue/river"
)

// const METRIC_COLLECTION_WORKER_INTERVAL = 24 * time.Hour // Run daily
const METRICS_COLLECTION_WORKER_INTERVAL = 10 * time.Second // Run every minute for testing

type MetricsCollectionUsecase interface {
	CollectMetrics(ctx context.Context) (map[string]any, error)
}

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
		&river.PeriodicJobOpts{RunOnStart: true}, // Don't run immediately on startup
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

func (w MetricCollectionWorker) Work(ctx context.Context, job *river.Job[models.MetricsCollectionArgs]) error {
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, "Starting metrics collection job")

	// Create the metric collection usecase
	metrics, err := w.collectMetrics(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to collect metrics", "error", err)
		return err
	}

	// Debug: Print the collected metrics
	logger.InfoContext(ctx, "Collected metrics", "metrics", metrics)

	// TODO: Store or send the metrics somewhere
	// For now, just log the number of metrics collected
	logger.InfoContext(ctx, "Metric collection completed", "metrics_count", len(metrics))

	return nil
}

func (w *MetricCollectionWorker) collectMetrics(ctx context.Context) (map[string]any, error) {
	metrics := make(map[string]any)

	// Collect global metrics
	globalMetrics, err := w.collectGlobalMetrics(ctx)
	if err != nil {
		return nil, err
	}
	maps.Copy(metrics, globalMetrics)

	// Collect organization-specific metrics
	orgMetrics, err := w.collectOrganizationMetrics(ctx)
	if err != nil {
		return nil, err
	}
	maps.Copy(metrics, orgMetrics)

	return metrics, nil
}

func (w *MetricCollectionWorker) collectGlobalMetrics(ctx context.Context) (map[string]any, error) {
	metrics := make(map[string]any)

	for _, collector := range w.collectors.GetGlobalCollectors() {
		value, err := collector.Collect(ctx)
		if err != nil {
			return nil, err
		}
		metrics[collector.Name()] = value
	}

	return metrics, nil
}

func (w *MetricCollectionWorker) collectOrganizationMetrics(ctx context.Context) (map[string]any, error) {
	metrics := make(map[string]any)

	orgs, err := w.getListOfOrganizations(ctx)
	if err != nil {
		return nil, err
	}

	for _, org := range orgs {
		for _, collector := range w.collectors.GetCollectors() {
			value, err := collector.Collect(ctx, org.Id)
			if err != nil {
				return nil, err
			}
			// Use a composite key to avoid conflicts between organizations
			// TODO: Store the organization ID in the metrics attributes
			key := collector.Name() + "_" + org.Id
			metrics[key] = value
		}
	}

	return metrics, nil
}

func (w *MetricCollectionWorker) getListOfOrganizations(ctx context.Context) ([]models.Organization, error) {
	orgs, err := w.organizationRepository.AllOrganizations(ctx, w.executorFactory.NewExecutor())
	if err != nil {
		return nil, err
	}
	return orgs, nil
}
