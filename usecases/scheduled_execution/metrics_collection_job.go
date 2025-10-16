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
				Queue: models.METRICS_QUEUE_NAME,
				UniqueOpts: river.UniqueOpts{
					ByQueue:  true,
					ByPeriod: config.JobInterval,
				},
				MaxAttempts: 1, // No retries
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
	SaveWatermark(ctx context.Context, exec repositories.Executor,
		orgId *string, watermarkType models.WatermarkType, watermarkId *string, watermarkTime time.Time, params json.RawMessage) error
}

type MetricCollectionWorker struct {
	river.WorkerDefaults[models.MetricsCollectionArgs]

	collectors      MetricsCollectionUsecase
	repository      WatermarkRepository
	executorFactory executor_factory.ExecutorFactory
	config          infra.MetricCollectionConfig
}

func NewMetricCollectionWorker(
	collectors MetricsCollectionUsecase,
	repository WatermarkRepository,
	executorFactory executor_factory.ExecutorFactory,
	config infra.MetricCollectionConfig,
) MetricCollectionWorker {
	return MetricCollectionWorker{
		collectors:      collectors,
		repository:      repository,
		executorFactory: executorFactory,
		config:          config,
	}
}

func (w MetricCollectionWorker) Timeout(job *river.Job[models.MetricsCollectionArgs]) time.Duration {
	return time.Minute
}

// Work executes the metrics collection job by collecting both global and organization-specific metrics
// from a time range defined by watermarks, then updates the watermark to track progress
func (w MetricCollectionWorker) Work(ctx context.Context, job *river.Job[models.MetricsCollectionArgs]) error {
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Starting metrics collection job")

	now := time.Now().UTC()

	// Take a watermark for the "from" time
	from := w.getFromTime(ctx, w.config.FallbackDuration).UTC()

	logger.DebugContext(ctx, "Collecting metrics from", "from", from)
	// Create the metric collection usecase
	metricsCollection, err := w.collectors.CollectMetrics(ctx, from, now)
	if err != nil {
		return err
	}

	logger.DebugContext(ctx, "Sending metrics to ingestion")
	if err := w.sendMetricsToIngestion(ctx, metricsCollection); err != nil {
		return err
	}

	logger.DebugContext(ctx, "Updating watermarks", "new_timestamp", now)
	if err := w.repository.SaveWatermark(ctx, w.executorFactory.NewExecutor(), nil,
		models.WatermarkTypeMetrics, nil, now, nil); err != nil {
		return err
	}

	return nil
}

// Get the from time from the watermark table, always return a Time in UTC
// In case there is no watermark, collect metrics from the last fallbackDuration defined in the config
func (w MetricCollectionWorker) getFromTime(ctx context.Context, fallbackDuration time.Duration) time.Time {
	exec := w.executorFactory.NewExecutor()
	watermark, err := w.repository.GetWatermark(ctx, exec, nil, models.WatermarkTypeMetrics)
	if err != nil || watermark == nil || watermark.WatermarkTime.IsZero() {
		return time.Now().UTC().Add(-fallbackDuration)
	}
	return watermark.WatermarkTime
}

func (w MetricCollectionWorker) sendMetricsToIngestion(ctx context.Context, metricsCollection models.MetricsCollection) error {
	return retry.Do(
		func() error {
			return w.doHTTPRequest(ctx, metricsCollection)
		},
		retry.Attempts(3),
		retry.LastErrorOnly(true),
		retry.Delay(100*time.Millisecond),
	)
}

func (w MetricCollectionWorker) doHTTPRequest(ctx context.Context, metricsCollection models.MetricsCollection) error {
	metricsCollectionDto := dto.AdaptMetricsCollectionDto(metricsCollection)

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(metricsCollectionDto); err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		w.config.MetricsIngestionURL, &body)
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
