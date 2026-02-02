package worker_jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

const (
	WEBHOOK_DELIVERY_TIMEOUT = 5 * time.Minute

	// DefaultMaxAttempts is the maximum number of delivery attempts before giving up.
	DefaultMaxAttempts = 24
)

// DefaultRetryDelays for webhook delivery failures (exponential backoff).
var DefaultRetryDelays = []time.Duration{
	30 * time.Second,  // Fast first retry for transient issues
	2 * time.Minute,   // Attempt 3
	10 * time.Minute,  // Attempt 4
	1 * time.Hour,     // Attempt 5
	4 * time.Hour,     // Attempt 6
	12 * time.Hour,    // Attempt 7+
}

// CalculateBackoff returns the delay for the next retry attempt.
func CalculateBackoff(attempt int) time.Duration {
	if attempt < 1 {
		return 0
	}

	idx := attempt - 1 // attempt 1 uses delay[0]
	if idx >= len(DefaultRetryDelays) {
		return DefaultRetryDelays[len(DefaultRetryDelays)-1]
	}
	return DefaultRetryDelays[idx]
}

// WebhookSendResult contains the result of a webhook delivery attempt.
type WebhookSendResult struct {
	StatusCode int
	Error      error
}

// IsSuccess returns true if the status code indicates success (2xx).
func (r WebhookSendResult) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// webhookDeliveryRepository defines the interface for webhook delivery operations.
type webhookDeliveryRepository interface {
	GetDelivery(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.WebhookDelivery, error)
	GetWebhook(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.NewWebhook, error)
	GetWebhookQueueItem(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.WebhookQueueItem, error)
	ListActiveSecrets(ctx context.Context, exec repositories.Executor, webhookId uuid.UUID) ([]models.NewWebhookSecret, error)
	UpdateDeliverySuccess(ctx context.Context, exec repositories.Executor, id uuid.UUID, responseStatus int) error
	UpdateDeliveryFailed(ctx context.Context, exec repositories.Executor, id uuid.UUID, errMsg string, responseStatus *int) error
	UpdateDeliveryAttempt(ctx context.Context, exec repositories.Executor, id uuid.UUID, errMsg string, responseStatus *int, attempts int, nextRetryAt time.Time) error
}

// webhookDeliveryTaskQueue defines the interface for scheduling retry jobs.
type webhookDeliveryTaskQueue interface {
	EnqueueWebhookDeliveryAt(ctx context.Context, tx repositories.Transaction, organizationId uuid.UUID, deliveryId uuid.UUID, scheduledAt time.Time) error
}

// webhookOrganizationRepository defines the interface for getting organization data.
type webhookOrganizationRepository interface {
	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId uuid.UUID) (models.Organization, error)
}

// WebhookDeliveryServiceFunc is a function type for webhook delivery to avoid import cycles.
type WebhookDeliveryServiceFunc func(ctx context.Context, webhook models.NewWebhook, secrets []models.NewWebhookSecret, event models.WebhookQueueItem) WebhookSendResult

// WebhookDeliveryWorker is the Stage 2 worker that delivers webhooks to individual endpoints.
type WebhookDeliveryWorker struct {
	river.WorkerDefaults[models.WebhookDeliveryJobArgs]

	webhookRepository  webhookDeliveryRepository
	orgRepository      webhookOrganizationRepository
	taskQueue          webhookDeliveryTaskQueue
	deliveryFunc       WebhookDeliveryServiceFunc
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	maxAttempts        int
}

// NewWebhookDeliveryWorker creates a new webhook delivery worker.
func NewWebhookDeliveryWorker(
	webhookRepository webhookDeliveryRepository,
	orgRepository webhookOrganizationRepository,
	taskQueue webhookDeliveryTaskQueue,
	deliveryFunc WebhookDeliveryServiceFunc,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
) *WebhookDeliveryWorker {
	return &WebhookDeliveryWorker{
		webhookRepository:  webhookRepository,
		orgRepository:      orgRepository,
		taskQueue:          taskQueue,
		deliveryFunc:       deliveryFunc,
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		maxAttempts:        DefaultMaxAttempts,
	}
}

func (w *WebhookDeliveryWorker) Timeout(job *river.Job[models.WebhookDeliveryJobArgs]) time.Duration {
	return WEBHOOK_DELIVERY_TIMEOUT
}

// Work processes a webhook delivery job by sending the webhook to the endpoint.
func (w *WebhookDeliveryWorker) Work(ctx context.Context, job *river.Job[models.WebhookDeliveryJobArgs]) error {
	logger := utils.LoggerFromContext(ctx).With(
		"delivery_id", job.Args.DeliveryId,
	)
	ctx = utils.StoreLoggerInContext(ctx, logger)

	exec := w.executorFactory.NewExecutor()

	// Get the delivery record
	delivery, err := w.webhookRepository.GetDelivery(ctx, exec, job.Args.DeliveryId)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get delivery", "error", err)
		return err // Infrastructure error → River retries
	}

	logger = logger.With(
		"webhook_id", delivery.WebhookId,
		"webhook_event_id", delivery.WebhookEventId,
		"attempt", delivery.Attempts+1,
	)

	// Already completed (idempotency check)
	if delivery.Status != models.WebhookDeliveryStatusPending {
		logger.DebugContext(ctx, "Delivery already completed", "status", delivery.Status)
		return nil
	}

	// Get the webhook configuration
	webhook, err := w.webhookRepository.GetWebhook(ctx, exec, delivery.WebhookId)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get webhook", "error", err)
		return err // Infrastructure error → River retries
	}

	// Get the event data
	event, err := w.webhookRepository.GetWebhookQueueItem(ctx, exec, delivery.WebhookEventId)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get webhook event", "error", err)
		return err // Infrastructure error → River retries
	}

	// Get active secrets for signing
	secrets, err := w.webhookRepository.ListActiveSecrets(ctx, exec, webhook.Id)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get webhook secrets", "error", err)
		return err // Infrastructure error → River retries
	}

	logger.InfoContext(ctx, "Delivering webhook",
		"url", webhook.Url,
		"event_type", event.EventType)

	// Attempt HTTP delivery
	result := w.deliveryFunc(ctx, webhook, secrets, event)

	// Increment attempt count
	newAttempts := delivery.Attempts + 1

	if result.IsSuccess() {
		// Success - mark delivery as complete
		logger.InfoContext(ctx, "Webhook delivered successfully", "status_code", result.StatusCode)
		return w.webhookRepository.UpdateDeliverySuccess(ctx, exec, delivery.Id, result.StatusCode)
	}

	// HTTP failure - format error message
	errMsg := w.formatError(result)
	var statusCode *int
	if result.StatusCode > 0 {
		statusCode = &result.StatusCode
	}

	logger.WarnContext(ctx, "Webhook delivery failed",
		"error", errMsg,
		"status_code", result.StatusCode,
		"attempts", newAttempts,
		"max_attempts", w.maxAttempts)

	// Check if we've exhausted retries
	if newAttempts >= w.maxAttempts {
		logger.ErrorContext(ctx, "Webhook delivery exhausted all retries",
			"attempts", newAttempts)
		return w.webhookRepository.UpdateDeliveryFailed(ctx, exec, delivery.Id, errMsg, statusCode)
	}

	// Schedule retry with backoff
	nextRetryAt := time.Now().Add(CalculateBackoff(newAttempts))

	// Update delivery record with attempt info
	err = w.webhookRepository.UpdateDeliveryAttempt(ctx, exec, delivery.Id, errMsg, statusCode, newAttempts, nextRetryAt)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to update delivery attempt", "error", err)
		return err
	}

	// Get organization for queue routing
	org, err := w.orgRepository.GetOrganizationById(ctx, exec, event.OrganizationId)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get organization", "error", err)
		return err
	}

	// Enqueue new job scheduled for later
	err = w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		return w.taskQueue.EnqueueWebhookDeliveryAt(ctx, tx, org.Id, delivery.Id, nextRetryAt)
	})
	if err != nil {
		logger.ErrorContext(ctx, "Failed to enqueue retry job", "error", err)
		return err // If enqueue fails, River retries this job
	}

	logger.InfoContext(ctx, "Scheduled webhook retry",
		"next_retry_at", nextRetryAt,
		"attempts", newAttempts)

	// Return nil because we manage retries ourselves, not River
	return nil
}

// formatError creates a human-readable error message from the delivery result.
func (w *WebhookDeliveryWorker) formatError(result WebhookSendResult) string {
	if result.Error != nil {
		return fmt.Sprintf("HTTP error: %s", result.Error.Error())
	}
	return fmt.Sprintf("HTTP status: %d", result.StatusCode)
}
