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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

const (
	MAX_CONCURRENT_WEBHOOKS_SENT      = 20
	WEBHOOKS_SEND_MAX_RETRIES         = 24
	DEFAULT_FAILED_WEBHOOKS_PAGE_SIZE = 1000
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
	UpdateWebhookEvent(
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
	transactionFactory          executor_factory.TransactionFactory
	convoyRepository            convoyWebhookEventRepository
	webhookEventsRepository     webhookEventsRepository
	failedWebhooksRetryPageSize int
}

func NewWebhookEventsUsecase(
	enforceSecurity enforceSecurityWebhookEvents,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	convoyRepository convoyWebhookEventRepository,
	webhookEventsRepository webhookEventsRepository,
	failedWebhooksRetryPageSize int,
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
		failedWebhooksRetryPageSize: failedWebhooksRetryPageSize,
	}
}

func (usecase WebhookEventsUsecase) CreateWebhookEvent(
	ctx context.Context,
	tx repositories.Executor,
	input models.WebhookEventCreate,
) error {
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

// SendWebhookEventAsync sends a webhook event asynchronously, with a new context and timeout and a child span.
func (usecase WebhookEventsUsecase) SendWebhookEventAsync(ctx context.Context, webhookEventId string) {
	logger := utils.LoggerFromContext(ctx).With("webhook_event_id", webhookEventId)
	ctx = utils.StoreLoggerInContext(ctx, logger)

	// go routine to send the webhook event asynchronously, with a new context and timeout and a child span
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second*3)
		defer cancel()
		tracer := utils.OpenTelemetryTracerFromContext(ctx)
		ctx, span := tracer.Start(
			ctx,
			"CreateWebhookEvent.SendWebhookEventAsync",
			trace.WithAttributes(attribute.String("webhook_event_id", webhookEventId)))
		defer span.End()

		_, err := usecase._sendWebhookEvent(ctx, webhookEventId)
		if err != nil {
			logger.ErrorContext(ctx, fmt.Sprintf("Error sending webhook event %s: %s", webhookEventId, err.Error()))
		}
	}()
}

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
	logger.InfoContext(ctx, fmt.Sprintf("Found %d webhook events to send", len(pendingWebhookEvents)))
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
	for _, status := range deliveryStatuses {
		switch status {
		case models.Success:
			successCount++
		case models.Retry:
			retryCount++
		}
	}
	logger.InfoContext(ctx, fmt.Sprintf("Webhook events sent: %d success, %d retry out of %d events",
		successCount, retryCount, len(pendingWebhookEvents)))

	return nil
}

// _sendWebhookEvent actually sends a webhook event and updates its status in the database.
func (usecase WebhookEventsUsecase) _sendWebhookEvent(ctx context.Context, webhookEventId string) (models.WebhookEventDeliveryStatus, error) {
	logger := utils.LoggerFromContext(ctx)
	exec := usecase.executorFactory.NewExecutor()
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

	err = usecase.convoyRepository.SendWebhookEvent(ctx, webhookEvent)
	// sending the webhook to convoy was successful, so we can update the status directly (no need for transaction)
	if err == nil {
		webhookEventUpdate := models.WebhookEventUpdate{Id: webhookEvent.Id, DeliveryStatus: models.Success}
		err := usecase.webhookEventsRepository.UpdateWebhookEvent(ctx, exec, webhookEventUpdate)
		return webhookEventUpdate.DeliveryStatus, err
	}

	// so we're in the error case
	logger.ErrorContext(ctx, fmt.Sprintf("Error sending webhook event %s: %s", webhookEvent.Id, err.Error()))

	// there was an error sending the webhook to convoy so we mark it as failed or to retry
	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) (models.WebhookEventDeliveryStatus, error) {
			webhookEventBeforeUpdate, err := usecase.webhookEventsRepository.GetWebhookEvent(ctx, tx, webhookEventId)
			if err != nil {
				return models.Scheduled, err
			}
			// another goroutine already updated the webhook event, no need to do anything else
			if webhookEventBeforeUpdate.DeliveryStatus == models.Success {
				return webhookEventBeforeUpdate.DeliveryStatus, nil
			}

			webhookEventUpdate := models.WebhookEventUpdate{
				Id:             webhookEvent.Id,
				DeliveryStatus: models.Retry,
				RetryCount:     webhookEvent.RetryCount + 1,
			}
			err = usecase.webhookEventsRepository.UpdateWebhookEvent(ctx, tx, webhookEventUpdate)
			return webhookEventUpdate.DeliveryStatus, errors.Wrapf(
				err,
				"error while updating webhook event %s", webhookEvent.Id,
			)
		})
}
