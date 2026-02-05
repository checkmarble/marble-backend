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
	WEBHOOK_CLEANUP_INTERVAL = 1 * time.Hour
	WEBHOOK_CLEANUP_TIMEOUT  = 5 * time.Minute
	// WEBHOOK_RETENTION_PERIOD = 30 * 24 * time.Hour // 30 days
	WEBHOOK_RETENTION_PERIOD = 5 * time.Second // 30 days
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
	DeleteOldWebhookDeliveriesBatch(ctx context.Context, exec repositories.Executor,
		olderThan time.Time, limit int) (int64, error)
	DeleteOrphanedWebhookEventsV2Batch(ctx context.Context, exec repositories.Executor, limit int) (int64, error)
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

// Work cleans up old webhook deliveries and orphaned events in batches.
func (w *WebhookCleanupWorker) Work(ctx context.Context, job *river.Job[models.WebhookCleanupJobArgs]) error {
	logger := utils.LoggerFromContext(ctx)
	exec := w.executorFactory.NewExecutor()

	cutoff := time.Now().Add(-w.retentionPeriod)

	// Delete old deliveries in batches
	var totalDeliveries int64
	for {
		deleted, err := w.webhookRepository.DeleteOldWebhookDeliveriesBatch(ctx, exec, cutoff, w.batchSize)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to delete old webhook deliveries", "error", err)
			return err
		}
		totalDeliveries += deleted
		if deleted < int64(w.batchSize) {
			break
		}
	}

	// Delete orphaned events in batches
	var totalEvents int64
	for {
		deleted, err := w.webhookRepository.DeleteOrphanedWebhookEventsV2Batch(ctx, exec, w.batchSize)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to delete orphaned webhook events", "error", err)
			return err
		}
		totalEvents += deleted
		if deleted < int64(w.batchSize) {
			break
		}
	}

	if totalDeliveries > 0 || totalEvents > 0 {
		logger.InfoContext(ctx, "Webhook cleanup completed",
			"deleted_deliveries", totalDeliveries,
			"deleted_events", totalEvents,
			"retention_days", int(w.retentionPeriod.Hours()/24))
	}

	return nil
}
