package worker_jobs

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

const (
	WEBHOOK_DISPATCH_TIMEOUT = 2 * time.Minute
)

// webhookRepository defines the interface for webhook operations needed by dispatch worker.
type webhookRepository interface {
	GetWebhookQueueItem(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.WebhookQueueItem, error)
	ListWebhooksByEventType(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, eventType string) ([]models.NewWebhook, error)
	DeliveryExists(ctx context.Context, exec repositories.Executor, webhookEventId, webhookId uuid.UUID) (bool, error)
	CreateDelivery(ctx context.Context, exec repositories.Executor, delivery models.WebhookDelivery) error
}

// webhookTaskQueue defines the interface for enqueueing webhook delivery jobs.
type webhookTaskQueue interface {
	EnqueueWebhookDelivery(ctx context.Context, tx repositories.Transaction, organizationId uuid.UUID, deliveryId uuid.UUID) error
}

// WebhookDispatchWorker is the Stage 1 worker that fans out events to matching webhooks.
type WebhookDispatchWorker struct {
	river.WorkerDefaults[models.WebhookDispatchJobArgs]

	webhookRepository  webhookRepository
	taskQueue          webhookTaskQueue
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
}

// NewWebhookDispatchWorker creates a new webhook dispatch worker.
func NewWebhookDispatchWorker(
	webhookRepository webhookRepository,
	taskQueue webhookTaskQueue,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
) *WebhookDispatchWorker {
	return &WebhookDispatchWorker{
		webhookRepository:  webhookRepository,
		taskQueue:          taskQueue,
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
	}
}

func (w *WebhookDispatchWorker) Timeout(job *river.Job[models.WebhookDispatchJobArgs]) time.Duration {
	return WEBHOOK_DISPATCH_TIMEOUT
}

// Work processes a webhook dispatch job by finding matching webhooks and creating delivery records.
func (w *WebhookDispatchWorker) Work(ctx context.Context, job *river.Job[models.WebhookDispatchJobArgs]) error {
	logger := utils.LoggerFromContext(ctx).With(
		"webhook_event_id", job.Args.WebhookEventId,
	)
	ctx = utils.StoreLoggerInContext(ctx, logger)

	exec := w.executorFactory.NewExecutor()

	// Get the event from the queue
	event, err := w.webhookRepository.GetWebhookQueueItem(ctx, exec, job.Args.WebhookEventId)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get webhook queue item", "error", err)
		return err // Infrastructure error → River retries
	}

	logger = logger.With(
		"organization_id", event.OrganizationId,
		"event_type", event.EventType,
	)

	// Find matching webhooks for this organization and event type
	webhooks, err := w.webhookRepository.ListWebhooksByEventType(ctx, exec, event.OrganizationId, event.EventType)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list webhooks by event type", "error", err)
		return err // Infrastructure error → River retries
	}

	if len(webhooks) == 0 {
		logger.DebugContext(ctx, "No webhooks found for event type")
		return nil
	}

	logger.InfoContext(ctx, "Dispatching event to webhooks", "webhook_count", len(webhooks))

	// Create delivery records and enqueue delivery jobs for each webhook
	for _, webhook := range webhooks {
		// Idempotent: skip if delivery already exists (handles job retries)
		exists, err := w.webhookRepository.DeliveryExists(ctx, exec, event.Id, webhook.Id)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to check delivery existence", "webhook_id", webhook.Id, "error", err)
			return err // Infrastructure error → River retries
		}
		if exists {
			logger.DebugContext(ctx, "Delivery already exists, skipping", "webhook_id", webhook.Id)
			continue
		}

		// Create delivery record and enqueue job in a transaction
		deliveryId, err := uuid.NewV7()
		if err != nil {
			logger.ErrorContext(ctx, "Failed to generate delivery ID", "error", err)
			return err
		}

		err = w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
			delivery := models.WebhookDelivery{
				Id:             deliveryId,
				WebhookEventId: event.Id,
				WebhookId:      webhook.Id,
				Status:         models.WebhookDeliveryStatusPending,
				Attempts:       0,
			}

			if err := w.webhookRepository.CreateDelivery(ctx, tx, delivery); err != nil {
				return err
			}

			return w.taskQueue.EnqueueWebhookDelivery(ctx, tx, event.OrganizationId, deliveryId)
		})
		if err != nil {
			logger.ErrorContext(ctx, "Failed to create delivery", "webhook_id", webhook.Id, "error", err)
			return err // Infrastructure error → River retries
		}

		logger.DebugContext(ctx, "Created delivery", "webhook_id", webhook.Id, "delivery_id", deliveryId)
	}

	return nil
}
