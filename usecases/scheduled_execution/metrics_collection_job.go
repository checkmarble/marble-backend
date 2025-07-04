package scheduled_execution

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/riverqueue/river"
)

func NewMetricsCollectionPeriodicJob(config infra.MetricCollectionConfig) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(config.JobInterval),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.MetricsCollectionArgs{}, &river.InsertOpts{
				Queue: "metrics",
				UniqueOpts: river.UniqueOpts{
					ByQueue:  true,
					ByPeriod: config.JobInterval,
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
		orgId *string, watermarkType models.WatermarkType, watermarkId *string, watermarkTime time.Time, params json.RawMessage) error
}

type MetricCollectionWorker struct {
	river.WorkerDefaults[models.MetricsCollectionArgs]

	collectors         MetricsCollectionUsecase
	repository         WatermarkRepository
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	config             infra.MetricCollectionConfig
}

func NewMetricCollectionWorker(
	collectors MetricsCollectionUsecase,
	repository WatermarkRepository,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	config infra.MetricCollectionConfig,
) MetricCollectionWorker {
	return MetricCollectionWorker{
		collectors:         collectors,
		repository:         repository,
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		config:             config,
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

	logger.DebugContext(ctx, "Sending metrics to ingestion", "metrics", metricsCollection)
	if err := w.sendMetricsToIngestion(ctx, metricsCollection); err != nil {
		logger.WarnContext(ctx, "Failed to send metrics to ingestion", "error", err)
		return err
	}

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

func (w MetricCollectionWorker) sendMetricsToIngestion(ctx context.Context, metricsCollection models.MetricsCollection) error {
	f := func() error {
		metricsCollectionDto := dto.AdaptMetricsCollectionDto(metricsCollection)
		jsonData, err := json.Marshal(metricsCollectionDto)
		if err != nil {
			return err
		}

		request, err := http.NewRequestWithContext(ctx, "POST",
			w.config.MetricsIngestionURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(request)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return errors.Newf("unexpected status code from ingestion: %d", resp.StatusCode)
		}
		return nil
	}

	err := retry.Do(
		f,
		retry.Attempts(3),
		retry.LastErrorOnly(true),
		retry.Delay(100*time.Millisecond),
	)

	return err
}
