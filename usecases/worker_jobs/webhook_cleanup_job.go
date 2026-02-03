package worker_jobs

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/riverqueue/river"
)

const (
	WEBHOOK_CLEANUP_INTERVAL  = 1 * time.Hour
	WEBHOOK_CLEANUP_TIMEOUT   = 5 * time.Minute
	WEBHOOK_RETENTION_PERIOD  = 30 * 24 * time.Hour // 30 days
	WEBHOOK_CLEANUP_QUEUE     = "webhook_cleanup"
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
		&river.PeriodicJobOpts{RunOnStart: false},
	)
}

// webhookCleanupRepository defines the interface for cleanup operations.
type webhookCleanupRepository interface {
	DeleteOldWebhookDeliveries(ctx context.Context, exec repositories.Executor, olderThan time.Time) (int64, error)
	DeleteOrphanedWebhookEventsV2(ctx context.Context, exec repositories.Executor) (int64, error)
}

// WebhookCleanupWorker handles cleanup of old webhook deliveries and events.
type WebhookCleanupWorker struct {
	river.WorkerDefaults[models.WebhookCleanupJobArgs]

	webhookRepository webhookCleanupRepository
	executorFactory   executor_factory.ExecutorFactory
	retentionPeriod   time.Duration
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
	}
}

func (w *WebhookCleanupWorker) Timeout(job *river.Job[models.WebhookCleanupJobArgs]) time.Duration {
	return WEBHOOK_CLEANUP_TIMEOUT
}

// Work cleans up old webhook deliveries and orphaned events.
func (w *WebhookCleanupWorker) Work(ctx context.Context, job *river.Job[models.WebhookCleanupJobArgs]) error {
	logger := utils.LoggerFromContext(ctx)
	exec := w.executorFactory.NewExecutor()

	cutoff := time.Now().Add(-w.retentionPeriod)

	// Delete old deliveries (success/failed older than retention period)
	deletedDeliveries, err := w.webhookRepository.DeleteOldWebhookDeliveries(ctx, exec, cutoff)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to delete old webhook deliveries", "error", err)
		return err
	}

	// Delete orphaned events (no associated deliveries)
	deletedEvents, err := w.webhookRepository.DeleteOrphanedWebhookEventsV2(ctx, exec)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to delete orphaned webhook events", "error", err)
		return err
	}

	if deletedDeliveries > 0 || deletedEvents > 0 {
		logger.InfoContext(ctx, "Webhook cleanup completed",
			"deleted_deliveries", deletedDeliveries,
			"deleted_events", deletedEvents,
			"retention_days", int(w.retentionPeriod.Hours()/24))
	}

	return nil
}
