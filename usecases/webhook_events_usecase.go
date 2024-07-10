package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/guregu/null/v5"
	"github.com/pkg/errors"
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
		webhookEventId string,
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
	enforceSecurity         enforceSecurityWebhookEvents
	executorFactory         executor_factory.ExecutorFactory
	transactionFactory      executor_factory.TransactionFactory
	convoyRepository        convoyWebhookEventRepository
	webhookEventsRepository webhookEventsRepository
}

func NewWebhookEventsUsecase(
	enforceSecurity enforceSecurityWebhookEvents,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	convoyRepository convoyWebhookEventRepository,
	webhookEventsRepository webhookEventsRepository,
) WebhookEventsUsecase {
	return WebhookEventsUsecase{
		enforceSecurity:         enforceSecurity,
		executorFactory:         executorFactory,
		transactionFactory:      transactionFactory,
		convoyRepository:        convoyRepository,
		webhookEventsRepository: webhookEventsRepository,
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

	webhookEventId := uuid.New().String()
	err = usecase.webhookEventsRepository.CreateWebhookEvent(ctx,
		tx,
		webhookEventId,
		input,
	)
	if err != nil {
		return errors.Wrap(err, "error creating webhook event")
	}

	return nil
}

func (usecase WebhookEventsUsecase) SendWebhookEvents(
	ctx context.Context,
) error {
	logger := utils.LoggerFromContext(ctx)
	exec := usecase.executorFactory.NewExecutor()

	pendingWebhookEvents, err := usecase.webhookEventsRepository.ListWebhookEvents(ctx, exec, models.WebhookEventFilters{
		DeliveryStatus: []models.WebhookEventDeliveryStatus{models.Scheduled, models.Retry},
		Limit:          100,
	})
	if err != nil {
		return errors.Wrap(err, "error while listing pending webhook events")
	}
	logger.InfoContext(ctx, fmt.Sprintf("Found %d webhook events to send", len(pendingWebhookEvents)))
	if len(pendingWebhookEvents) == 0 {
		return nil
	}

	var waitGroup sync.WaitGroup
	// The channel needs to be big enough to store any possible errors to avoid deadlock due to the presence of a waitGroup
	uploadErrorChan := make(chan error, len(pendingWebhookEvents))
	deliveryStatusChan := make(chan models.WebhookEventDeliveryStatus, len(pendingWebhookEvents))

	startProcessSendWebhookEvent := func(webhookEventId string) {
		defer waitGroup.Done()
		logger := logger.With("webhook_event_id", webhookEventId)
		deliveryStatus, err := usecase.sendWebhookEvent(ctx, webhookEventId, logger)
		if err != nil {
			uploadErrorChan <- err
		}
		if deliveryStatus != nil {
			deliveryStatusChan <- *deliveryStatus
		}
	}

	for _, webhookEvent := range pendingWebhookEvents {
		waitGroup.Add(1)
		go startProcessSendWebhookEvent(webhookEvent.Id)
	}

	waitGroup.Wait()
	close(uploadErrorChan)
	close(deliveryStatusChan)

	errorCount := 0
	var firstError error
	for err := range uploadErrorChan {
		errorCount++
		if firstError == nil {
			firstError = err
		}
	}

	successCount := 0
	retryCount := 0
	failedCount := 0
	for status := range deliveryStatusChan {
		switch status {
		case models.Success:
			successCount++
		case models.Retry:
			retryCount++
		case models.Failed:
			failedCount++
		}
	}
	logger.InfoContext(ctx, fmt.Sprintf("Webhook events sent: %d success, %d retry, %d failed, %d errors",
		successCount, retryCount, failedCount, errorCount))

	return firstError
}

// sendWebhookEvent sends a webhook event and updates its status in the database.
// It returns the delivery status of the webhook event and an error if updating the webhook event fails.
func (usecase *WebhookEventsUsecase) sendWebhookEvent(
	ctx context.Context,
	webhookEventId string,
	logger *slog.Logger,
) (*models.WebhookEventDeliveryStatus, error) {
	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) (*models.WebhookEventDeliveryStatus, error) {
			webhookEvent, err := usecase.webhookEventsRepository.GetWebhookEvent(ctx, tx, webhookEventId)
			if err != nil {
				return nil, err
			}

			err = usecase.enforceSecurity.SendWebhookEvent(ctx, webhookEvent.OrganizationId, webhookEvent.PartnerId)
			if err != nil {
				return nil, err
			}

			logger.InfoContext(ctx, fmt.Sprintf("Start processing webhook event %s", webhookEvent.Id))

			err = usecase.convoyRepository.SendWebhookEvent(ctx, webhookEvent)
			webhookEventUpdate := models.WebhookEventUpdate{
				Id:               webhookEvent.Id,
				SendAttemptCount: webhookEvent.SendAttemptCount + 1,
			}
			if err == nil {
				webhookEventUpdate.DeliveryStatus = models.Success
			} else {
				if webhookEventUpdate.SendAttemptCount >= 3 {
					webhookEventUpdate.DeliveryStatus = models.Failed
				} else {
					webhookEventUpdate.DeliveryStatus = models.Retry
				}
			}
			err = usecase.webhookEventsRepository.UpdateWebhookEvent(ctx, tx, webhookEventUpdate)
			if err != nil {
				return nil, errors.Wrap(err, fmt.Sprintf(
					"error while updating webhook event %s", webhookEvent.Id))
			}
			return &webhookEventUpdate.DeliveryStatus, nil
		})
}
