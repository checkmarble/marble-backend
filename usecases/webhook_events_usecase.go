package usecases

import (
	"context"
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

type webhookEventsWebhookRepository interface {
	ListWebhooksByEventType(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, partnerId *uuid.UUID, eventType string) ([]models.Webhook, error)
	GetWebhook(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.Webhook, error)
	ListActiveSecrets(ctx context.Context, exec repositories.Executor, webhookId uuid.UUID) ([]models.Secret, error)
	CreateWebhookDeliveries(ctx context.Context, exec repositories.Executor, eventId uuid.UUID, webhookIds []uuid.UUID) ([]models.WebhookDelivery, error)
	ListWebhookDeliveriesForEvent(ctx context.Context, exec repositories.Executor, eventId uuid.UUID) ([]models.WebhookDelivery, error)
	ListPendingWebhookDeliveries(ctx context.Context, exec repositories.Executor, limit int) ([]models.WebhookDelivery, error)
	ListPendingWebhookDeliveriesForOrg(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, limit int) ([]models.WebhookDelivery, error)
	GetWebhookDelivery(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.WebhookDelivery, error)
	MarkWebhookDeliverySuccess(ctx context.Context, exec repositories.Executor, id uuid.UUID, responseStatus int) error
	MarkWebhookDeliveryFailed(ctx context.Context, exec repositories.Executor, id uuid.UUID, errMsg string, responseStatus *int, nextRetryAt *time.Time, attempts int) error
}

type enforceSecurityWebhookEvents interface {
	SendWebhookEvent(ctx context.Context, organizationId uuid.UUID, partnerId null.String) error
}

type WebhookEventsUsecase struct {
	enforceSecurity             enforceSecurityWebhookEvents
	executorFactory             executor_factory.ExecutorFactory
	webhookEventsRepository     webhookEventsRepository
	webhookRepository           webhookEventsWebhookRepository
	deliveryService             *WebhookDeliveryService
	failedWebhooksRetryPageSize int
	hasLicense                  bool
	publicApiAdaptor            types.PublicApiDataAdapter
}

func NewWebhookEventsUsecase(
	enforceSecurity enforceSecurityWebhookEvents,
	executorFactory executor_factory.ExecutorFactory,
	webhookEventsRepository webhookEventsRepository,
	webhookRepository webhookEventsWebhookRepository,
	deliveryService *WebhookDeliveryService,
	failedWebhooksRetryPageSize int,
	hasLicense bool,
	publicApiAdaptor types.PublicApiDataAdapter,
) WebhookEventsUsecase {
	if failedWebhooksRetryPageSize == 0 {
		failedWebhooksRetryPageSize = DEFAULT_FAILED_WEBHOOKS_PAGE_SIZE
	}

	return WebhookEventsUsecase{
		enforceSecurity:             enforceSecurity,
		executorFactory:             executorFactory,
		webhookEventsRepository:     webhookEventsRepository,
		webhookRepository:           webhookRepository,
		deliveryService:             deliveryService,
		failedWebhooksRetryPageSize: failedWebhooksRetryPageSize,
		hasLicense:                  hasLicense,
		publicApiAdaptor:            publicApiAdaptor,
	}
}

// Does nothing (returns nil) if the license is not active
func (usecase WebhookEventsUsecase) CreateWebhookEvent(
	ctx context.Context,
	tx repositories.Transaction,
	input models.WebhookEventCreate,
) error {
	if !usecase.hasLicense {
		return nil
	}

	err := usecase.enforceSecurity.SendWebhookEvent(ctx, input.OrganizationId, input.PartnerId)
	if err != nil {
		return err
	}

	// Create the webhook event
	err = usecase.webhookEventsRepository.CreateWebhookEvent(ctx, tx, input)
	if err != nil {
		return errors.Wrap(err, "error creating webhook event")
	}

	// Find matching webhooks for this org and event type
	var partnerIdPtr *uuid.UUID
	if input.PartnerId.Valid {
		id, err := uuid.Parse(input.PartnerId.String)
		if err == nil {
			partnerIdPtr = &id
		}
	}

	webhooks, err := usecase.webhookRepository.ListWebhooksByEventType(
		ctx, tx, input.OrganizationId, partnerIdPtr, string(input.EventContent.Type))
	if err != nil {
		return errors.Wrap(err, "error listing matching webhooks")
	}

	if len(webhooks) == 0 {
		return nil
	}

	// Create delivery records for each matching webhook
	webhookIds := make([]uuid.UUID, len(webhooks))
	for i, w := range webhooks {
		webhookIds[i] = w.Id
	}

	eventId, err := uuid.Parse(input.Id)
	if err != nil {
		return errors.Wrap(err, "invalid webhook event id")
	}

	_, err = usecase.webhookRepository.CreateWebhookDeliveries(ctx, tx, eventId, webhookIds)
	if err != nil {
		return errors.Wrap(err, "error creating webhook deliveries")
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

		err := usecase._sendWebhookEventDeliveries(ctx, webhookEventId)
		if err != nil {
			logger.ErrorContext(ctx, fmt.Sprintf("Error sending webhook event %s: %s", webhookEventId, err.Error()))
		}
	}()
}

// _sendWebhookEventDeliveries delivers a webhook event to all its pending deliveries
func (usecase WebhookEventsUsecase) _sendWebhookEventDeliveries(ctx context.Context, webhookEventId string) error {
	logger := utils.LoggerFromContext(ctx)
	exec := usecase.executorFactory.NewExecutor()

	if !usecase.hasLicense {
		logger.DebugContext(ctx, "Skipping webhook delivery - no license")
		return nil
	}

	webhookEvent, err := usecase.webhookEventsRepository.GetWebhookEvent(ctx, exec, webhookEventId)
	if err != nil {
		return err
	}

	err = usecase.enforceSecurity.SendWebhookEvent(ctx, webhookEvent.OrganizationId, webhookEvent.PartnerId)
	if err != nil {
		return err
	}

	eventId, err := uuid.Parse(webhookEventId)
	if err != nil {
		return errors.Wrap(err, "invalid webhook event id")
	}

	deliveries, err := usecase.webhookRepository.ListWebhookDeliveriesForEvent(ctx, exec, eventId)
	if err != nil {
		return errors.Wrap(err, "error listing deliveries for event")
	}

	if len(deliveries) == 0 {
		logger.DebugContext(ctx, "No pending deliveries for webhook event")
		return nil
	}

	// Build payload
	data, err := dto.AdaptWebhookEventData(ctx, exec, usecase.publicApiAdaptor, webhookEvent.EventContent.Data)
	if err != nil {
		return errors.Wrap(err, "error adapting webhook event data")
	}

	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(MAX_CONCURRENT_WEBHOOKS_SENT)

	for _, delivery := range deliveries {
		if delivery.Status != models.DeliveryPending {
			continue
		}

		delivery := delivery
		group.Go(func() error {
			return usecase.deliveryService.DeliverWebhook(ctx, delivery, webhookEvent, data)
		})
	}

	return group.Wait()
}

// RetrySendWebhookEvents retries sending webhook events that have failed to be sent.
// It handles them in limited batches.
func (usecase WebhookEventsUsecase) RetrySendWebhookEvents(
	ctx context.Context,
) error {
	exec := usecase.executorFactory.NewExecutor()

	pendingDeliveries, err := usecase.webhookRepository.ListPendingWebhookDeliveries(ctx, exec, usecase.failedWebhooksRetryPageSize)
	if err != nil {
		return errors.Wrap(err, "error while listing pending webhook deliveries")
	}

	return usecase.sendPendingDeliveries(ctx, pendingDeliveries)
}

// RetrySendWebhookEventsForOrg retries sending webhook events for a specific organization.
func (usecase WebhookEventsUsecase) RetrySendWebhookEventsForOrg(
	ctx context.Context,
	orgId uuid.UUID,
) error {
	exec := usecase.executorFactory.NewExecutor()

	pendingDeliveries, err := usecase.webhookRepository.ListPendingWebhookDeliveriesForOrg(ctx, exec, orgId, usecase.failedWebhooksRetryPageSize)
	if err != nil {
		return errors.Wrap(err, "error while listing pending webhook deliveries for org")
	}

	return usecase.sendPendingDeliveries(ctx, pendingDeliveries)
}

func (usecase WebhookEventsUsecase) sendPendingDeliveries(
	ctx context.Context,
	pendingDeliveries []models.WebhookDelivery,
) error {
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, fmt.Sprintf("Found %d webhook deliveries to retry", len(pendingDeliveries)))
	if len(pendingDeliveries) == 0 {
		return nil
	}

	exec := usecase.executorFactory.NewExecutor()

	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(MAX_CONCURRENT_WEBHOOKS_SENT)

	successCount := 0
	retryCount := 0
	failedCount := 0

	for _, delivery := range pendingDeliveries {
		delivery := delivery
		group.Go(func() error {
			ctx := utils.StoreLoggerInContext(
				ctx,
				logger.With("delivery_id", delivery.Id, "webhook_event_id", delivery.WebhookEventId))

			select {
			case <-ctx.Done():
				return errors.Wrapf(ctx.Err(), "context cancelled before retrying delivery %s", delivery.Id)
			default:
			}

			// Get the webhook event
			webhookEvent, err := usecase.webhookEventsRepository.GetWebhookEvent(ctx, exec, delivery.WebhookEventId.String())
			if err != nil {
				logger.ErrorContext(ctx, fmt.Sprintf("Failed to get webhook event for delivery %s: %s", delivery.Id, err.Error()))
				return nil // Don't fail the entire batch
			}

			// Build payload
			data, err := dto.AdaptWebhookEventData(ctx, exec, usecase.publicApiAdaptor, webhookEvent.EventContent.Data)
			if err != nil {
				logger.ErrorContext(ctx, fmt.Sprintf("Failed to adapt webhook data for delivery %s: %s", delivery.Id, err.Error()))
				return nil
			}

			err = usecase.deliveryService.DeliverWebhook(ctx, delivery, webhookEvent, data)
			if err != nil {
				logger.WarnContext(ctx, fmt.Sprintf("Delivery %s failed: %s", delivery.Id, err.Error()))
				retryCount++
			} else {
				successCount++
			}

			return nil
		})
	}

	err := group.Wait()
	if err != nil {
		return errors.Wrap(err, "error while sending webhook deliveries")
	}

	logger.InfoContext(ctx, fmt.Sprintf("Webhook deliveries processed: %d success, %d retry, %d failed out of %d deliveries",
		successCount, retryCount, failedCount, len(pendingDeliveries)))

	return nil
}
