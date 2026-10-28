package worker_jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

func NewMetricsCollectionPeriodicJob(config infra.MetricCollectionConfig) *river.PeriodicJob {
	return NewPeriodicJob(
		river.PeriodicInterval(config.JobInterval),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.MetricsCollectionArgs{}, &river.InsertOpts{
				Queue: models.METRICS_QUEUE_NAME,
				UniqueOpts: river.UniqueOpts{
					ByQueue: true,
					// Unique across unfinished states only: a finished run must not block the next one,
					// but a pending or running one must (a leadership change would otherwise start an overlapping run).
					ByState: []rivertype.JobState{
						rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRetryable,
						rivertype.JobStateRunning, rivertype.JobStateScheduled,
					},
				},
				MaxAttempts: 1, // No retries
			}
		},
	)
}

type MetricsCollectionUsecase interface {
	CounterCollectorNames() []string
	CollectMetrics(ctx context.Context, req models.MetricsCollectionRequest) (models.MetricsCollectionResult, error)
}

type WatermarkRepository interface {
	GetWatermark(ctx context.Context, exec repositories.Executor, orgId *uuid.UUID,
		watermarkType models.WatermarkType) (*models.Watermark, error)
	SaveWatermark(ctx context.Context, exec repositories.Executor,
		orgId *uuid.UUID, watermarkType models.WatermarkType, watermarkId *string,
		watermarkTime time.Time, params json.RawMessage) error
}

type MetricCollectionWorker struct {
	river.WorkerDefaults[models.MetricsCollectionArgs]

	collectors         MetricsCollectionUsecase
	repository         WatermarkRepository
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	config             infra.MetricCollectionConfig
	now                func() time.Time
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
		now:                time.Now,
	}
}

func (w MetricCollectionWorker) Timeout(job *river.Job[models.MetricsCollectionArgs]) time.Duration {
	return w.config.JobTimeout
}

// Work collects metrics for each counter collector from its own watermark (the snapshot collectors use
// the legacy one), sends them to the ingestion endpoint, and only then saves the watermarks. A collector
// failure does not fail the job: its watermark simply stops at the last completed period.
func (w MetricCollectionWorker) Work(ctx context.Context, job *river.Job[models.MetricsCollectionArgs]) error {
	logger := utils.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Starting metrics collection job")

	now := w.now().UTC()

	snapshotFrom, counterFrom, err := w.resolveWatermarks(ctx, now)
	if err != nil {
		return err
	}

	collectCtx, cancel := context.WithTimeout(ctx, w.config.CollectionBudget())
	result, err := w.collectors.CollectMetrics(collectCtx, models.MetricsCollectionRequest{
		Now:              now,
		SnapshotFrom:     snapshotFrom,
		CounterFrom:      counterFrom,
		CollectorTimeout: w.config.CollectorTimeout,
	})
	cancel()
	if err != nil {
		return err
	}

	for _, p := range result.Progress {
		if p.Err != nil {
			logger.WarnContext(ctx, "metrics collector did not complete",
				"collector", p.CollectorName,
				"from", p.From,
				"completed_until", p.CompletedUntil,
				"error", p.Err.Error())
			if !p.CompletedUntil.After(p.From) {
				// Stuck: no progress this run. errors.Handled hides the context.DeadlineExceeded/Canceled cause,
				// which LogAndReportSentryError would otherwise drop at Debug level.
				utils.LogAndReportSentryError(ctx, errors.Wrapf(errors.Handled(p.Err),
					"metrics collector %s stuck: made no progress", p.CollectorName))
			}
			continue
		}
		logger.InfoContext(ctx, "metrics collector progress",
			"collector", p.CollectorName,
			"from", p.From,
			"completed_until", p.CompletedUntil)
	}

	// Sending and saving (with its own context below) must survive a cancellation of the job context: the collected data would
	// otherwise be lost or sent without its watermark being saved.
	sendCtx, cancelSend := context.WithTimeout(context.WithoutCancel(ctx), infra.METRICS_SEND_TIMEOUT)
	defer cancelSend()

	logger.InfoContext(ctx, "Sending metrics to ingestion", "metrics", len(result.Collection.Metrics))
	if err := w.sendMetricsToIngestion(sendCtx, result.Collection); err != nil {
		// errors.Handled so that a send timeout is reported to Sentry rather than dropped as a deadline error.
		return errors.Wrap(errors.Handled(err), "failed to send metrics to ingestion")
	}

	saveCtx, cancelSave := context.WithTimeout(context.WithoutCancel(ctx), infra.METRICS_SAVE_TIMEOUT)
	defer cancelSave()

	logger.DebugContext(ctx, "Updating watermarks", "new_timestamp", now)
	err = w.transactionFactory.Transaction(saveCtx, func(tx repositories.Transaction) error {
		// Driven by every requested counter, not by the returned progress: a counter without progress keeps
		// its starting value (own watermark or fallback) so that it is never lost.
		completedUntil := make(map[string]time.Time, len(result.Progress))
		for _, p := range result.Progress {
			completedUntil[p.CollectorName] = p.CompletedUntil
		}
		for _, name := range slices.Sorted(maps.Keys(counterFrom)) {
			until, ok := completedUntil[name]
			if !ok {
				until = counterFrom[name]
			}
			if err := w.repository.SaveWatermark(saveCtx, tx, nil,
				models.SpecializedWatermark(models.WatermarkTypeMetrics, name),
				nil, until, nil); err != nil {
				return errors.Wrapf(err, "failed to save watermark for collector %s", name)
			}
		}
		return w.repository.SaveWatermark(saveCtx, tx, nil, models.WatermarkTypeMetrics, nil, now, nil)
	})
	if err != nil {
		return errors.Wrap(err, "failed to save metrics watermarks")
	}

	return nil
}

// resolveWatermarks returns the "from" time for the snapshot collectors (legacy watermark) and for
// each counter collector (own watermark only, seeded from the legacy one by a migration). Only a
// missing watermark falls back to now - FallbackDuration; a read error fails the job, since falling back
// would re-send already ingested data. All times are UTC and never after now.
func (w MetricCollectionWorker) resolveWatermarks(ctx context.Context, now time.Time) (
	snapshotFrom time.Time, counterFrom map[string]time.Time, err error,
) {
	exec := w.executorFactory.NewExecutor()

	snapshotFrom = now.Add(-w.config.FallbackDuration)
	legacy, err := w.repository.GetWatermark(ctx, exec, nil, models.WatermarkTypeMetrics)
	if err != nil {
		return time.Time{}, nil, errors.Wrap(err, "failed to get metrics watermark")
	}
	if legacy != nil && !legacy.WatermarkTime.IsZero() {
		snapshotFrom = legacy.WatermarkTime.UTC()
	}
	if snapshotFrom.After(now) {
		snapshotFrom = now
	}

	names := w.collectors.CounterCollectorNames()
	counterFrom = make(map[string]time.Time, len(names))
	for _, name := range names {
		wm, err := w.repository.GetWatermark(ctx, exec, nil,
			models.SpecializedWatermark(models.WatermarkTypeMetrics, name))
		if err != nil {
			return time.Time{}, nil, errors.Wrapf(err, "failed to get metrics watermark for collector %s", name)
		}
		from := now.Add(-w.config.FallbackDuration)
		if wm != nil && !wm.WatermarkTime.IsZero() {
			from = wm.WatermarkTime.UTC()
		}
		if from.After(now) {
			from = now
		}
		counterFrom[name] = from
	}

	return snapshotFrom, counterFrom, nil
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
