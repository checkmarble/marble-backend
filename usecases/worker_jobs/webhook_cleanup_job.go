package worker_jobs

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/riverqueue/river"
)

const (
	WEBHOOK_CLEANUP_INTERVAL = 1 * time.Hour
	WEBHOOK_CLEANUP_TIMEOUT  = 5 * time.Minute
	WEBHOOK_RETENTION_PERIOD = 30 * 24 * time.Hour // 30 days
	WEBHOOK_CLEANUP_QUEUE    = "webhook_cleanup"
	WEBHOOK_CLEANUP_BATCH    = 1000
)

func NewWebhookCleanupPeriodicJob() *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(WEBHOOK_CLEANUP_INTERVAL),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.WebhookCleanupJobArgs{},
				&river.InsertOpts{
					Queue:    WEBHOOK_CLEANUP_QUEUE,
					Priority: 4, // Low priority
					UniqueOpts: river.UniqueOpts{
						ByQueue:  true,
						ByPeriod: WEBHOOK_CLEANUP_INTERVAL,
					},
				}
		},
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}

// webhookCleanupRepository defines the interface for cleanup operations.
type webhookCleanupRepository interface {
	DeleteOldWebhookEventsV2Batch(ctx context.Context, exec repositories.Executor,
		olderThan time.Time, limit int) (int64, error)
}

// WebhookCleanupWorker handles cleanup of old webhook deliveries and events.
type WebhookCleanupWorker struct {
	river.WorkerDefaults[models.WebhookCleanupJobArgs]

	webhookRepository webhookCleanupRepository
	executorFactory   executor_factory.ExecutorFactory
	retentionPeriod   time.Duration
	batchSize         int
}

// NewWebhookCleanupWorker creates a new webhook cleanup worker.
func NewWebhookCleanupWorker(
	webhookRepository webhookCleanupRepository,
	executorFactory executor_factory.ExecutorFactory,
) *WebhookCleanupWorker {
	return &WebhookCleanupWorker{
		webhookRepository: webhookRepository,
		executorFactory:   executorFactory,
		retentionPeriod:   WEBHOOK_RETENTION_PERIOD,
		batchSize:         WEBHOOK_CLEANUP_BATCH,
	}
}

func (w *WebhookCleanupWorker) Timeout(job *river.Job[models.WebhookCleanupJobArgs]) time.Duration {
	return WEBHOOK_CLEANUP_TIMEOUT
}

// Work cleans up old webhook events in batches.
// Deliveries are cascade-deleted via FK constraint.
func (w *WebhookCleanupWorker) Work(ctx context.Context, job *river.Job[models.WebhookCleanupJobArgs]) error {
	logger := utils.LoggerFromContext(ctx)
	exec := w.executorFactory.NewExecutor()

	cutoff := time.Now().Add(-w.retentionPeriod)

	var totalEvents int64
	for {
		deleted, err := w.webhookRepository.DeleteOldWebhookEventsV2Batch(ctx, exec, cutoff, w.batchSize)
		if err != nil {
			return errors.Wrap(err, "failed to delete old webhook events")
		}
		totalEvents += deleted
		if deleted < int64(w.batchSize) {
			break
		}
	}

	if totalEvents > 0 {
		logger.InfoContext(ctx, "Webhook cleanup completed",
			"deleted_events", totalEvents,
			"retention_days", int(w.retentionPeriod.Hours()/24))
	}

	return nil
}
