package scheduled_execution

import (
	"context"
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
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
				Queue: "metrics",
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

type WatermarkRepository interface {
	GetWatermark(ctx context.Context, exec repositories.Executor, orgId *string,
		watermarkType models.WatermarkType) (*models.Watermark, error)
	SaveWatermark(ctx context.Context, tx repositories.Transaction,
		orgId *string, watermarkType models.WatermarkType, watermarkId *string, watermarkTime time.Time, params *json.RawMessage) error
}

type MetricCollectionWorker struct {
	river.WorkerDefaults[models.MetricsCollectionArgs]

	collectors         MetricsCollectionUsecase
	repository         WatermarkRepository
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
}

func NewMetricCollectionWorker(
	collectors MetricsCollectionUsecase,
	repository WatermarkRepository,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
) MetricCollectionWorker {
	return MetricCollectionWorker{
		collectors:         collectors,
		repository:         repository,
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
	}
}

// Work executes the metrics collection job by collecting both global and organization-specific metrics
// from a time range defined by watermarks, then updates the watermark to track progress
func (w MetricCollectionWorker) Work(ctx context.Context, job *river.Job[models.MetricsCollectionArgs]) error {
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Starting metrics collection job")

	now := time.Now().UTC()

	// Take a watermark for the "from" time
	from := w.getFromTime(ctx).UTC()
	logger.DebugContext(ctx, "Collecting metrics from", "from", from)

	// Create the metric collection usecase
	metricsCollection, err := w.collectors.CollectMetrics(ctx, from, now)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to collect metrics", "error", err)
		return err
	}

	logger.DebugContext(ctx, "Collected metrics", "metrics", metricsCollection)

	// TODO: Update the watermarks with now value
	logger.DebugContext(ctx, "Updating watermarks", "new_timestamp", now)
	if err := w.saveWatermark(ctx, now); err != nil {
		logger.ErrorContext(ctx, "Failed to save watermark", "error", err)
		return err
	}

	// TODO: Store or send the metrics somewhere
	// For now, just log the number of metrics collected
	logger.DebugContext(ctx, "Metric collection completed", "metrics_count", len(metricsCollection.Metrics))

	return nil
}

// Get the from time from the watermark table, always return a Time in UTC
func (w MetricCollectionWorker) getFromTime(ctx context.Context) time.Time {
	exec := w.executorFactory.NewExecutor()
	watermark, err := w.repository.GetWatermark(ctx, exec, nil, models.WatermarkTypeMetrics)
	if err != nil || watermark == nil || watermark.WatermarkTime.IsZero() {
		return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	return watermark.WatermarkTime
}

func (w MetricCollectionWorker) saveWatermark(ctx context.Context, newWatermarkTime time.Time) error {
	return w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		return w.repository.SaveWatermark(ctx, tx, nil, models.WatermarkTypeMetrics, nil, newWatermarkTime, nil)
	})
}
