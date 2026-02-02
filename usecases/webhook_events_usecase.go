package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/guregu/null/v5"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

const (
	MAX_CONCURRENT_WEBHOOKS_SENT      = 20
	WEBHOOKS_SEND_MAX_RETRIES         = 24
	DEFAULT_FAILED_WEBHOOKS_PAGE_SIZE = 1000
	ASYNC_WEBHOOKS_SEND_TIMEOUT       = 5 * time.Minute
)

type convoyWebhookEventRepository interface {
	SendWebhookEvent(ctx context.Context, webhookEvent models.WebhookEvent, apiVersion string, payload json.RawMessage) error
}

type webhookEventsRepository interface {
	GetWebhookEvent(ctx context.Context, exec repositories.Executor, webhookEventId string) (models.WebhookEvent, error)
	ListWebhookEvents(
		ctx context.Context,
		exec repositories.Executor,
		filters models.WebhookEventFilters,
	) ([]models.WebhookEvent, error)
	CreateWebhookEvent(
		ctx context.Context,
		exec repositories.Executor,
		webhookEvent models.WebhookEventCreate,
	) error
	MarkWebhookEventRetried(
		ctx context.Context,
		exec repositories.Executor,
		input models.WebhookEventUpdate,
	) error
}

type enforceSecurityWebhookEvents interface {
	SendWebhookEvent(ctx context.Context, organizationId uuid.UUID, partnerId null.String) error
}

// webhookQueueRepository is the interface for the new webhook queue system.
type webhookQueueRepository interface {
	CreateWebhookQueueItem(ctx context.Context, exec repositories.Executor, item models.WebhookQueueItem) error
}

// webhookTaskQueue is the interface for enqueueing webhook dispatch jobs.
type webhookTaskQueue interface {
	EnqueueWebhookDispatch(ctx context.Context, tx repositories.Transaction, organizationId uuid.UUID, webhookEventId uuid.UUID) error
}

type WebhookEventsUsecase struct {
	enforceSecurity             enforceSecurityWebhookEvents
	executorFactory             executor_factory.ExecutorFactory
	transactionFactory          executor_factory.TransactionFactory
	convoyRepository            convoyWebhookEventRepository
	webhookEventsRepository     webhookEventsRepository
	webhookQueueRepository      webhookQueueRepository
	taskQueue                   webhookTaskQueue
	failedWebhooksRetryPageSize int
	hasLicense                  bool
	hasConvoyServerSetup        bool
	useNewWebhooks              bool
	publicApiAdaptor            types.PublicApiDataAdapter
}

func NewWebhookEventsUsecase(
	enforceSecurity enforceSecurityWebhookEvents,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	convoyRepository convoyWebhookEventRepository,
	webhookEventsRepository webhookEventsRepository,
	webhookQueueRepository webhookQueueRepository,
	taskQueue webhookTaskQueue,
	failedWebhooksRetryPageSize int,
	hasLicense bool,
	hasConvoyServerSetup bool,
	useNewWebhooks bool,
	publicApiAdaptor types.PublicApiDataAdapter,
) WebhookEventsUsecase {
	if failedWebhooksRetryPageSize == 0 {
		failedWebhooksRetryPageSize = DEFAULT_FAILED_WEBHOOKS_PAGE_SIZE
	}

	return WebhookEventsUsecase{
		enforceSecurity:             enforceSecurity,
		executorFactory:             executorFactory,
		transactionFactory:          transactionFactory,
		convoyRepository:            convoyRepository,
		webhookEventsRepository:     webhookEventsRepository,
		webhookQueueRepository:      webhookQueueRepository,
		taskQueue:                   taskQueue,
		failedWebhooksRetryPageSize: failedWebhooksRetryPageSize,
		hasLicense:                  hasLicense,
		hasConvoyServerSetup:        hasConvoyServerSetup,
		useNewWebhooks:              useNewWebhooks,
		publicApiAdaptor:            publicApiAdaptor,
	}
}

// Does nothing (returns nil) if the license is not active.
// Routes to new webhook system if useNewWebhooks is enabled.
func (usecase WebhookEventsUsecase) CreateWebhookEvent(
	ctx context.Context,
	tx repositories.Transaction,
	input models.WebhookEventCreate,
) error {
	// Check license and setup
	if !usecase.hasLicense {
		return nil
	}

	// If new webhooks enabled, use new system; otherwise require Convoy setup
	if !usecase.useNewWebhooks && !usecase.hasConvoyServerSetup {
		return nil
	}

	err := usecase.enforceSecurity.SendWebhookEvent(ctx, input.OrganizationId, input.PartnerId)
	if err != nil {
		return err
	}

	// Route to new or old system based on feature flag
	if usecase.useNewWebhooks {
		return usecase.createWebhookQueueItem(ctx, tx, input)
	}

	// Legacy Convoy path
	err = usecase.webhookEventsRepository.CreateWebhookEvent(ctx, tx, input)
	if err != nil {
		return errors.Wrap(err, "error creating webhook event")
	}
	return nil
}

// createWebhookQueueItem creates an event in the new webhook queue and enqueues a dispatch job.
func (usecase WebhookEventsUsecase) createWebhookQueueItem(
	ctx context.Context,
	tx repositories.Transaction,
	input models.WebhookEventCreate,
) error {
	// Serialize event data to JSON
	eventData, err := json.Marshal(input.EventContent.Data)
	if err != nil {
		return errors.Wrap(err, "error marshaling webhook event data")
	}

	// Generate UUID v7 for the event
	eventId, err := uuid.NewV7()
	if err != nil {
		return errors.Wrap(err, "error generating webhook event ID")
	}

	// Create webhook queue item
	queueItem := models.WebhookQueueItem{
		Id:             eventId,
		OrganizationId: input.OrganizationId,
		EventType:      string(input.EventContent.Type),
		EventData:      eventData,
	}

	err = usecase.webhookQueueRepository.CreateWebhookQueueItem(ctx, tx, queueItem)
	if err != nil {
		return errors.Wrap(err, "error creating webhook queue item")
	}

	// Enqueue dispatch job (fan-out happens asynchronously)
	err = usecase.taskQueue.EnqueueWebhookDispatch(ctx, tx, input.OrganizationId, eventId)
	if err != nil {
		return errors.Wrap(err, "error enqueueing webhook dispatch job")
	}

	return nil
}

// SendWebhookEventAsync sends a webhook event asynchronously, with a new context and timeout. This is the public method that should be
// used from other usecases.
func (usecase WebhookEventsUsecase) SendWebhookEventAsync(ctx context.Context, webhookEventId string) {
	logger := utils.LoggerFromContext(ctx).With("webhook_event_id", webhookEventId)
	ctx = utils.StoreLoggerInContext(ctx, logger)

	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), ASYNC_WEBHOOKS_SEND_TIMEOUT)
		defer cancel()

		_, err := usecase._sendWebhookEvent(ctx, webhookEventId)
		if err != nil {
			logger.ErrorContext(ctx, fmt.Sprintf("Error sending webhook event %s: %s", webhookEventId, err.Error()))
		}
	}()
}

// RetrySendWebhookEvents retries sending webhook events that have failed to be sent.
// It handles them in limited batches.
// TODO: refactor the whole usecase to use the the task queue tu send webhooks, removing the need for those methods (usecases should
// just create a task transactionally)
func (usecase WebhookEventsUsecase) RetrySendWebhookEvents(
	ctx context.Context,
) error {
	exec := usecase.executorFactory.NewExecutor()

	pendingWebhookEvents, err := usecase.webhookEventsRepository.ListWebhookEvents(ctx, exec, models.WebhookEventFilters{
		DeliveryStatus: []models.WebhookEventDeliveryStatus{models.Scheduled, models.Retry},
		Limit:          uint64(usecase.failedWebhooksRetryPageSize),
	})
	if err != nil {
		return errors.Wrap(err, "error while listing pending webhook events")
	}

	return usecase.sendPendingWebhookEvents(ctx, pendingWebhookEvents)
}

// RetrySendWebhookEventsForOrg retries sending webhook events for a specific organization.
func (usecase WebhookEventsUsecase) RetrySendWebhookEventsForOrg(
	ctx context.Context,
	orgId uuid.UUID,
) error {
	exec := usecase.executorFactory.NewExecutor()

	pendingWebhookEvents, err := usecase.webhookEventsRepository.ListWebhookEvents(ctx, exec, models.WebhookEventFilters{
		DeliveryStatus: []models.WebhookEventDeliveryStatus{models.Scheduled, models.Retry},
		Limit:          uint64(usecase.failedWebhooksRetryPageSize),
		OrganizationId: &orgId,
	})
	if err != nil {
		return errors.Wrap(err, "error while listing pending webhook events for org")
	}

	return usecase.sendPendingWebhookEvents(ctx, pendingWebhookEvents)
}

func (usecase WebhookEventsUsecase) sendPendingWebhookEvents(
	ctx context.Context,
	pendingWebhookEvents []models.WebhookEvent,
) error {
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, fmt.Sprintf("Found %d webhook events to retry", len(pendingWebhookEvents)))
	if len(pendingWebhookEvents) == 0 {
		return nil
	}

	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(MAX_CONCURRENT_WEBHOOKS_SENT)

	deliveryStatuses := make([]models.WebhookEventDeliveryStatus, len(pendingWebhookEvents))

	for i, webhookEvent := range pendingWebhookEvents {
		group.Go(func() error {
			ctx := utils.StoreLoggerInContext(
				ctx,
				logger.With("webhook_event_id", webhookEvent.Id))

			select {
			case <-ctx.Done():
				return errors.Wrapf(ctx.Err(), "context cancelled before retrying webhook %s", webhookEvent.Id)
			default:
			}

			deliveryStatus, err := usecase._sendWebhookEvent(ctx, webhookEvent.Id)
			deliveryStatuses[i] = deliveryStatus
			return err
		})
	}

	err := group.Wait()
	if err != nil {
		return errors.Wrap(err, "error while sending webhook events")
	}

	successCount := 0
	retryCount := 0
	skipCount := 0
	for _, status := range deliveryStatuses {
		switch status {
		case models.Success:
			successCount++
		case models.Retry:
			retryCount++
		case models.Skipped:
			skipCount++
		}
	}
	logger.InfoContext(ctx, fmt.Sprintf("Webhook events sent: %d success, %d retry, %d skipped out of %d events",
		successCount, retryCount, skipCount, len(pendingWebhookEvents)))

	return nil
}

// _sendWebhookEvent actually sends a webhook event using the repository, and updates its status in the database.
// The webhook event is marked as "skipped" if the currently active license does not include webhooks, or if no Convoy instance has been configured.
func (usecase WebhookEventsUsecase) _sendWebhookEvent(ctx context.Context, webhookEventId string) (models.WebhookEventDeliveryStatus, error) {
	logger := utils.LoggerFromContext(ctx)
	exec := usecase.executorFactory.NewExecutor()
	if !usecase.hasLicense || !usecase.hasConvoyServerSetup {
		return models.Skipped, usecase.webhookEventsRepository.MarkWebhookEventRetried(
			ctx, exec, models.WebhookEventUpdate{Id: webhookEventId, DeliveryStatus: models.Skipped})
	}

	webhookEvent, err := usecase.webhookEventsRepository.GetWebhookEvent(ctx, exec, webhookEventId)
	if err != nil {
		return models.Scheduled, err
	}
	if webhookEvent.DeliveryStatus == models.Success {
		return webhookEvent.DeliveryStatus, nil
	}

	err = usecase.enforceSecurity.SendWebhookEvent(ctx, webhookEvent.OrganizationId, webhookEvent.PartnerId)
	if err != nil {
		return models.Scheduled, err
	}

	logger.DebugContext(ctx, fmt.Sprintf("Start processing webhook event %s", webhookEvent.Id))

	webhookEventUpdate := models.WebhookEventUpdate{Id: webhookEvent.Id}

	apiVersion, data, err := dto.AdaptWebhookEventData(ctx, exec, usecase.publicApiAdaptor, webhookEvent.EventContent.Data)
	if err != nil {
		return models.Scheduled, err
	}

	err = usecase.convoyRepository.SendWebhookEvent(ctx, webhookEvent, apiVersion, data)
	if err == nil {
		webhookEventUpdate.DeliveryStatus = models.Success
	} else {
		logger.ErrorContext(ctx, fmt.Sprintf("Error sending webhook event %s: %s", webhookEvent.Id, err.Error()))
		webhookEventUpdate.DeliveryStatus = models.Retry
	}

	err = usecase.webhookEventsRepository.MarkWebhookEventRetried(ctx, exec, webhookEventUpdate)
	if err != nil {
		return webhookEventUpdate.DeliveryStatus,
			errors.Wrapf(err, "error while updating webhook event %s", webhookEvent.Id)
	}
	return webhookEventUpdate.DeliveryStatus, nil
}
