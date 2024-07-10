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

type convoyRepository interface {
	SendWebhookEvent(ctx context.Context, webhook models.Webhook) error
}

type webhookRepository interface {
	GetWebhook(ctx context.Context, exec repositories.Executor, webhookId string) (models.Webhook, error)
	ListWebhooks(ctx context.Context, exec repositories.Executor, filters models.WebhookFilters) ([]models.Webhook, error)
	CreateWebhook(
		ctx context.Context,
		exec repositories.Executor,
		webhookId string,
		webhook models.WebhookCreate,
	) error
	UpdateWebhook(
		ctx context.Context,
		exec repositories.Executor,
		input models.WebhookUpdate,
	) error
}

type enforceSecurityWebhooks interface {
	CanManageWebhook(ctx context.Context, organizationId string, partnerId null.String) error
}

type WebhooksUsecase struct {
	enforceSecurity    enforceSecurityWebhooks
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	convoyRepository   convoyRepository
	webhookRepository  webhookRepository
}

func NewWebhooksUsecase(
	enforceSecurity enforceSecurityWebhooks,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	convoyRepository convoyRepository,
	webhookRepository webhookRepository,
) WebhooksUsecase {
	return WebhooksUsecase{
		enforceSecurity:    enforceSecurity,
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		convoyRepository:   convoyRepository,
		webhookRepository:  webhookRepository,
	}
}

func (usecase WebhooksUsecase) CreateWebhook(
	ctx context.Context,
	tx repositories.Executor,
	input models.WebhookCreate,
) error {
	err := usecase.enforceSecurity.CanManageWebhook(ctx, input.OrganizationId, input.PartnerId)
	if err != nil {
		return err
	}

	webhookId := uuid.New().String()
	err = usecase.webhookRepository.CreateWebhook(ctx,
		tx,
		webhookId,
		input,
	)
	if err != nil {
		return errors.Wrap(err, "error creating webhook")
	}

	return nil
}

func (usecase WebhooksUsecase) SendWebhooks(
	ctx context.Context,
) error {
	logger := utils.LoggerFromContext(ctx)
	exec := usecase.executorFactory.NewExecutor()

	pendingWebhooks, err := usecase.webhookRepository.ListWebhooks(ctx, exec, models.WebhookFilters{
		DeliveryStatus: []models.WebhookDeliveryStatus{models.Scheduled, models.Retry},
		Limit:          100,
	})
	if err != nil {
		return errors.Wrap(err, "error while listing pending webhooks")
	}
	logger.InfoContext(ctx, fmt.Sprintf("Found %d webhooks to send", len(pendingWebhooks)))
	if len(pendingWebhooks) == 0 {
		return nil
	}

	var waitGroup sync.WaitGroup
	// The channel needs to be big enough to store any possible errors to avoid deadlock due to the presence of a waitGroup
	uploadErrorChan := make(chan error, len(pendingWebhooks))
	deliveryStatusChan := make(chan models.WebhookDeliveryStatus, len(pendingWebhooks))

	startProcessSendWebhook := func(webhook models.Webhook) {
		defer waitGroup.Done()
		logger := logger.With("webhook_id", webhook.Id)
		deliveryStatus, err := usecase.sendWebhook(ctx, webhook, logger)
		if err != nil {
			uploadErrorChan <- err
		}
		if deliveryStatus != nil {
			deliveryStatusChan <- *deliveryStatus
		}
	}

	for _, webhook := range pendingWebhooks {
		waitGroup.Add(1)
		go startProcessSendWebhook(webhook)
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
	logger.InfoContext(ctx, fmt.Sprintf("Webhooks sent: %d success, %d retry, %d failed, %d errors",
		successCount, retryCount, failedCount, errorCount))

	return firstError
}

// sendWebhook sends a webhook and updates its status in the database.
// It returns the delivery status of the webhook and an error if updating the webhook fails.
func (usecase *WebhooksUsecase) sendWebhook(
	ctx context.Context,
	webhook models.Webhook,
	logger *slog.Logger,
) (*models.WebhookDeliveryStatus, error) {
	err := usecase.enforceSecurity.CanManageWebhook(ctx, webhook.OrganizationId, webhook.PartnerId)
	if err != nil {
		return nil, err
	}

	exec := usecase.executorFactory.NewExecutor()
	logger.InfoContext(ctx, fmt.Sprintf("Start processing webhook %s", webhook.Id))

	err = usecase.convoyRepository.SendWebhookEvent(ctx, webhook)

	webhookUpdate := models.WebhookUpdate{
		Id:               webhook.Id,
		SendAttemptCount: webhook.SendAttemptCount + 1,
	}
	if err == nil {
		webhookUpdate.DeliveryStatus = models.Success
	} else {
		if webhookUpdate.SendAttemptCount >= 3 {
			webhookUpdate.DeliveryStatus = models.Failed
		} else {
			webhookUpdate.DeliveryStatus = models.Retry
		}
	}
	err = usecase.webhookRepository.UpdateWebhook(ctx, exec, webhookUpdate)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("error while updating webhook %s", webhook.Id))
	}
	return &webhookUpdate.DeliveryStatus, nil
}
