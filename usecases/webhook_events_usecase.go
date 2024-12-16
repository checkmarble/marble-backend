package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
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
	SendWebhookEvent(ctx context.Context, webhookEvent models.WebhookEvent) error
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
	SendWebhookEvent(ctx context.Context, organizationId string, partnerId null.String) error
}

type WebhookEventsUsecase struct {
	enforceSecurity             enforceSecurityWebhookEvents
	executorFactory             executor_factory.ExecutorFactory
	convoyRepository            convoyWebhookEventRepository
	webhookEventsRepository     webhookEventsRepository
	failedWebhooksRetryPageSize int
	hasLicense                  bool
	hasConvoyServerSetup        bool
}

func NewWebhookEventsUsecase(
	enforceSecurity enforceSecurityWebhookEvents,
	executorFactory executor_factory.ExecutorFactory,
	convoyRepository convoyWebhookEventRepository,
	webhookEventsRepository webhookEventsRepository,
	failedWebhooksRetryPageSize int,
	hasLicense bool,
	hasConvoyServerSetup bool,
) WebhookEventsUsecase {
	if failedWebhooksRetryPageSize == 0 {
		failedWebhooksRetryPageSize = DEFAULT_FAILED_WEBHOOKS_PAGE_SIZE
	}

	return WebhookEventsUsecase{
		enforceSecurity:             enforceSecurity,
		executorFactory:             executorFactory,
		convoyRepository:            convoyRepository,
		webhookEventsRepository:     webhookEventsRepository,
		failedWebhooksRetryPageSize: failedWebhooksRetryPageSize,
		hasLicense:                  hasLicense,
		hasConvoyServerSetup:        hasConvoyServerSetup,
	}
}

// Does nothing (returns nil) if the license is not active
func (usecase WebhookEventsUsecase) CreateWebhookEvent(
	ctx context.Context,
	tx repositories.Transaction,
	input models.WebhookEventCreate,
) error {
	if !usecase.hasLicense || !usecase.hasConvoyServerSetup {
		return nil
	}

	err := usecase.enforceSecurity.SendWebhookEvent(ctx, input.OrganizationId, input.PartnerId)
	if err != nil {
		return err
	}

	err = usecase.webhookEventsRepository.CreateWebhookEvent(ctx, tx, input)
	if err != nil {
		return errors.Wrap(err, "error creating webhook event")
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
	logger := utils.LoggerFromContext(ctx)
	exec := usecase.executorFactory.NewExecutor()

	pendingWebhookEvents, err := usecase.webhookEventsRepository.ListWebhookEvents(ctx, exec, models.WebhookEventFilters{
		DeliveryStatus: []models.WebhookEventDeliveryStatus{models.Scheduled, models.Retry},
		Limit:          uint64(usecase.failedWebhooksRetryPageSize),
	})
	if err != nil {
		return errors.Wrap(err, "error while listing pending webhook events")
	}
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

	err = group.Wait()
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

	logger.InfoContext(ctx, fmt.Sprintf("Start processing webhook event %s", webhookEvent.Id))

	webhookEventUpdate := models.WebhookEventUpdate{Id: webhookEvent.Id}

	err = usecase.convoyRepository.SendWebhookEvent(ctx, webhookEvent)
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
