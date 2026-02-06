package worker_jobs

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

const (
	WEBHOOK_DISPATCH_TIMEOUT = 10 * time.Second
)

// webhookDispatchRepository defines the interface for webhook operations needed by dispatch worker.
type webhookDispatchRepository interface {
	GetWebhookEventV2(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.WebhookEventV2, error)
	ListWebhooksByEventType(ctx context.Context, exec repositories.Executor, orgId uuid.UUID,
		eventType string) ([]models.NewWebhook, error)
	WebhookDeliveryExists(ctx context.Context, exec repositories.Executor,
		webhookEventId, webhookId uuid.UUID) (bool, error)
	CreateWebhookDelivery(ctx context.Context, exec repositories.Executor, delivery models.WebhookDelivery) error
	DeleteWebhookEventV2(ctx context.Context, exec repositories.Executor, id uuid.UUID) error
}

// webhookDispatchTaskQueue defines the interface for enqueueing webhook delivery jobs.
type webhookDispatchTaskQueue interface {
	EnqueueWebhookDelivery(ctx context.Context, tx repositories.Transaction, organizationId uuid.UUID, deliveryId uuid.UUID) error
}

// WebhookDispatchWorker is the Stage 1 worker that fans out events to matching webhooks.
type WebhookDispatchWorker struct {
	river.WorkerDefaults[models.WebhookDispatchJobArgs]

	webhookRepository  webhookDispatchRepository
	taskQueue          webhookDispatchTaskQueue
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
}

// NewWebhookDispatchWorker creates a new webhook dispatch worker.
func NewWebhookDispatchWorker(
	webhookRepository webhookDispatchRepository,
	taskQueue webhookDispatchTaskQueue,
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
	logger := utils.LoggerFromContext(ctx).With("webhook_event_id", job.Args.WebhookEventId)
	ctx = utils.StoreLoggerInContext(ctx, logger)

	exec := w.executorFactory.NewExecutor()

	event, err := w.webhookRepository.GetWebhookEventV2(ctx, exec, job.Args.WebhookEventId)
	if err != nil {
		return errors.Wrap(err, "failed to get webhook event")
	}

	logger = logger.With("organization_id", event.OrganizationId, "event_type", event.EventType)
	ctx = utils.StoreLoggerInContext(ctx, logger)

	webhooks, err := w.webhookRepository.ListWebhooksByEventType(ctx, exec, event.OrganizationId, event.EventType)
	if err != nil {
		return errors.Wrap(err, "failed to list webhooks for event type")
	}

	if len(webhooks) == 0 {
		logger.DebugContext(ctx, "No webhooks found for event type, deleting event")
		return w.webhookRepository.DeleteWebhookEventV2(ctx, exec, event.Id)
	}

	logger.DebugContext(ctx, "Dispatching event to webhooks", "webhook_count", len(webhooks))

	// Single transaction for all deliveries (atomicity)
	err = w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		for _, webhook := range webhooks {
			// Idempotent: skip if delivery already exists (handles job retries)
			exists, err := w.webhookRepository.WebhookDeliveryExists(ctx, tx, event.Id, webhook.Id)
			if err != nil {
				return err
			}
			if exists {
				logger.DebugContext(ctx, "Delivery already exists, skipping", "webhook_id", webhook.Id)
				continue
			}

			deliveryId, err := uuid.NewV7()
			if err != nil {
				return err
			}

			delivery := models.WebhookDelivery{
				Id:             deliveryId,
				WebhookEventId: event.Id,
				WebhookId:      webhook.Id,
				Status:         models.WebhookDeliveryStatusPending,
				Attempts:       0,
			}

			if err := w.webhookRepository.CreateWebhookDelivery(ctx, tx, delivery); err != nil {
				return err
			}

			if err := w.taskQueue.EnqueueWebhookDelivery(ctx, tx, event.OrganizationId, deliveryId); err != nil {
				return err
			}

			logger.DebugContext(ctx, "Created delivery", "webhook_id", webhook.Id, "delivery_id", deliveryId)
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to create deliveries")
	}

	return nil
}
