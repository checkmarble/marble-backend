package scheduled_execution

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
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

type MetricsCollectionUsecase interface {
	CollectMetrics(ctx context.Context, from time.Time, to time.Time) (models.MetricsCollection, error)
}

type MetricCollectionWorker struct {
	river.WorkerDefaults[models.MetricsCollectionArgs]

	collectors MetricsCollectionUsecase
}

func NewMetricCollectionWorker(
	collectors MetricsCollectionUsecase,
) MetricCollectionWorker {
	return MetricCollectionWorker{
		collectors: collectors,
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
	metricsCollection, err := w.collectors.CollectMetrics(ctx, from, now)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to collect metrics", "error", err)
		return err
	}

	logger.DebugContext(ctx, "Collected metrics", "metrics", metricsCollection)

	// TODO: Update the watermarks with now value
	logger.DebugContext(ctx, "Updating watermarks", "new_timestamp", now)

	// TODO: Store or send the metrics somewhere
	// For now, just log the number of metrics collected
	logger.DebugContext(ctx, "Metric collection completed", "metrics_count", len(metricsCollection.Metrics))

	return nil
}
