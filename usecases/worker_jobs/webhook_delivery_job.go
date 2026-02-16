package worker_jobs

import (
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
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
	// Job timeout should be slightly larger than max HTTP timeout (30s) + overhead
	WEBHOOK_DELIVERY_TIMEOUT = 35 * time.Second

	// DefaultMaxAttempts is the maximum number of delivery attempts before giving up.
	DefaultMaxAttempts = 24
)

// DefaultRetryDelays for webhook delivery failures (exponential backoff).
var DefaultRetryDelays = []time.Duration{
	30 * time.Second, // Fast first retry for transient issues
	2 * time.Minute,
	10 * time.Minute,
	1 * time.Hour,
	4 * time.Hour,
	12 * time.Hour,
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
	GetWebhookDelivery(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.WebhookDelivery, error)
	GetWebhook(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.NewWebhook, error)
	GetWebhookEventV2(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.WebhookEventV2, error)
	ListActiveWebhookSecrets(ctx context.Context, exec repositories.Executor, webhookId uuid.UUID) ([]models.NewWebhookSecret, error)
	UpdateWebhookDeliverySuccess(ctx context.Context, exec repositories.Executor, id uuid.UUID, responseStatus int) error
	UpdateWebhookDeliveryFailed(ctx context.Context, exec repositories.Executor, id uuid.UUID,
		errMsg string, responseStatus *int) error
	UpdateWebhookDeliveryAttempt(ctx context.Context, exec repositories.Executor, id uuid.UUID,
		errMsg string, responseStatus *int, attempts int, nextRetryAt time.Time) error
}

// webhookDeliveryTaskQueue defines the interface for scheduling retry jobs.
type webhookDeliveryTaskQueue interface {
	EnqueueWebhookDeliveryAt(ctx context.Context, tx repositories.Transaction,
		organizationId uuid.UUID, deliveryId uuid.UUID, scheduledAt time.Time) error
}

// webhookOrganizationRepository defines the interface for getting organization data.
type webhookOrganizationRepository interface {
	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId uuid.UUID) (models.Organization, error)
}

// WebhookDeliveryServiceFunc is a function type for webhook delivery to avoid import cycles.
type WebhookDeliveryServiceFunc func(ctx context.Context, webhook models.NewWebhook,
	secrets []models.NewWebhookSecret, event models.WebhookEventV2) WebhookSendResult

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
	logger := utils.LoggerFromContext(ctx).With("delivery_id", job.Args.DeliveryId)
	ctx = utils.StoreLoggerInContext(ctx, logger)

	exec := w.executorFactory.NewExecutor()

	delivery, err := w.webhookRepository.GetWebhookDelivery(ctx, exec, job.Args.DeliveryId)
	if err != nil {
		return errors.Wrap(err, "failed to get delivery")
	}

	logger = logger.With(
		"webhook_id", delivery.WebhookId,
		"webhook_event_id", delivery.WebhookEventId,
		"attempt", delivery.Attempts+1,
	)
	ctx = utils.StoreLoggerInContext(ctx, logger)

	// Already completed (idempotency check)
	if delivery.Status != models.WebhookDeliveryStatusPending {
		logger.DebugContext(ctx, "Delivery already completed", "status", delivery.Status)
		return nil
	}

	webhook, err := w.webhookRepository.GetWebhook(ctx, exec, delivery.WebhookId)
	if err != nil {
		return errors.Wrap(err, "failed to get webhook for delivery")
	}

	event, err := w.webhookRepository.GetWebhookEventV2(ctx, exec, delivery.WebhookEventId)
	if err != nil {
		return errors.Wrap(err, "failed to get webhook event for delivery")
	}

	secrets, err := w.webhookRepository.ListActiveWebhookSecrets(ctx, exec, webhook.Id)
	if err != nil {
		return errors.Wrap(err, "failed to get webhook secrets for delivery")
	}

	logger.DebugContext(ctx, "Delivering webhook", "url", webhook.Url, "event_type", event.EventType)

	result := w.deliveryFunc(ctx, webhook, secrets, event)
	newAttempts := delivery.Attempts + 1

	if result.IsSuccess() {
		logger.DebugContext(ctx, "Webhook delivered successfully", "status_code", result.StatusCode)
		return w.webhookRepository.UpdateWebhookDeliverySuccess(ctx, exec, delivery.Id, result.StatusCode)
	}

	// Delivery failed
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

	if newAttempts >= w.maxAttempts {
		logger.WarnContext(ctx, "Webhook delivery exhausted all retries", "attempts", newAttempts)
		return w.webhookRepository.UpdateWebhookDeliveryFailed(ctx, exec, delivery.Id, errMsg, statusCode)
	}

	// Schedule retry with backoff
	nextRetryAt := time.Now().Add(CalculateBackoff(newAttempts))

	err = w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		err = w.webhookRepository.UpdateWebhookDeliveryAttempt(ctx, tx, delivery.Id,
			errMsg, statusCode, newAttempts, nextRetryAt)
		if err != nil {
			return errors.Wrap(err, "failed to update delivery attempt")
		}

		return w.taskQueue.EnqueueWebhookDeliveryAt(ctx, tx, event.OrganizationId, delivery.Id, nextRetryAt)
	})
	if err != nil {
		return errors.Wrap(err, "failed to enqueue retry job")
	}

	logger.DebugContext(ctx, "Scheduled webhook retry", "next_retry_at", nextRetryAt, "attempts", newAttempts)
	return nil
}

func (w *WebhookDeliveryWorker) formatError(result WebhookSendResult) string {
	if result.Error == nil {
		return fmt.Sprintf("HTTP %d", result.StatusCode)
	}

	err := result.Error

	// Check for context timeout/deadline exceeded
	if errors.Is(err, context.DeadlineExceeded) {
		return "Request timed out"
	}
	if errors.Is(err, context.Canceled) {
		return "Request was cancelled"
	}

	// Check for URL errors (wraps most network errors)
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err // unwrap to get the underlying error
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return "Connection timed out"
		}
	}

	// Check for DNS errors
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "DNS lookup failed: could not resolve hostname"
	}

	// Check for connection refused
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if strings.Contains(opErr.Error(), "connection refused") {
			return "Connection refused: endpoint is not accepting connections"
		}
		if strings.Contains(opErr.Error(), "no route to host") {
			return "No route to host: endpoint is unreachable"
		}
		if strings.Contains(opErr.Error(), "network is unreachable") {
			return "Network unreachable"
		}
	}

	// Check for TLS/certificate errors
	var certErr *x509.CertificateInvalidError
	if errors.As(err, &certErr) {
		return "TLS certificate error: certificate is invalid"
	}
	var unknownAuthErr x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthErr) {
		return "TLS certificate error: unknown certificate authority"
	}
	var hostnameErr x509.HostnameError
	if errors.As(err, &hostnameErr) {
		return "TLS certificate error: hostname mismatch"
	}
	if strings.Contains(err.Error(), "certificate") || strings.Contains(err.Error(), "x509") {
		return "TLS certificate error"
	}

	// Check for EOF (connection closed unexpectedly)
	if errors.Is(err, io.EOF) {
		return "Connection closed unexpectedly by the server"
	}

	// Fallback: return a sanitized version of the error
	// Avoid exposing internal details, but provide some context
	errStr := err.Error()
	if len(errStr) > 100 {
		errStr = errStr[:100] + "..."
	}
	return fmt.Sprintf("Request failed: %s", errStr)
}
