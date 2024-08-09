package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/guregu/null/v5"
	"github.com/pkg/errors"
)

type convoyWebhooksRepository interface {
	GetWebhook(ctx context.Context, webhookId string) (models.Webhook, error)
	ListWebhooks(ctx context.Context, organizationId string, partnerId null.String) ([]models.Webhook, error)
	RegisterWebhook(ctx context.Context, organizationId string, partnerId null.String,
		input models.WebhookRegister) (models.Webhook, error)
	UpdateWebhook(ctx context.Context, input models.Webhook) (models.Webhook, error)
	DeleteWebhook(ctx context.Context, webhookId string) error
}

type enforceSecurityWebhook interface {
	CanCreateWebhook(ctx context.Context, organizationId string, partnerId null.String) error
	CanReadWebhook(ctx context.Context, webhook models.Webhook) error
	CanModifyWebhook(ctx context.Context, webhook models.Webhook) error
}

type WebhooksUsecase struct {
	enforceSecurity    enforceSecurityWebhook
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	convoyRepository   convoyWebhooksRepository
}

func NewWebhooksUsecase(
	enforceSecurity enforceSecurityWebhook,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	convoyRepository convoyWebhooksRepository,
) WebhooksUsecase {
	return WebhooksUsecase{
		enforceSecurity:    enforceSecurity,
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		convoyRepository:   convoyRepository,
	}
}

func (usecase WebhooksUsecase) ListWebhooks(ctx context.Context, organizationId string, partnerId null.String) ([]models.Webhook, error) {
	webhooks, err := usecase.convoyRepository.ListWebhooks(ctx, organizationId, partnerId)
	if err != nil {
		return nil, errors.Wrap(err, "error listing webhooks")
	}

	for _, webhook := range webhooks {
		if err := usecase.enforceSecurity.CanReadWebhook(ctx, webhook); err != nil {
			return nil, err
		}
	}

	return webhooks, nil
}

func (usecase WebhooksUsecase) RegisterWebhook(
	ctx context.Context,
	organizationId string,
	partnerId null.String,
	input models.WebhookRegister,
) (models.Webhook, error) {
	err := usecase.enforceSecurity.CanCreateWebhook(ctx, organizationId, partnerId)
	if err != nil {
		return models.Webhook{}, err
	}

	if err = input.Validate(); err != nil {
		return models.Webhook{}, err
	}

	webhook, err := usecase.convoyRepository.RegisterWebhook(ctx, organizationId, partnerId, input)
	return webhook, errors.Wrap(err, "error registering webhook")
}

func (usecase WebhooksUsecase) GetWebhook(
	ctx context.Context, organizationId string, partnerId null.String, webhookId string,
) (models.Webhook, error) {
	webhook, err := usecase.convoyRepository.GetWebhook(ctx, webhookId)
	if err != nil {
		return models.Webhook{}, models.NotFoundError
	}
	if err = usecase.enforceSecurity.CanReadWebhook(ctx, webhook); err != nil {
		return models.Webhook{}, err
	}
	return webhook, nil
}

func (usecase WebhooksUsecase) DeleteWebhook(
	ctx context.Context, organizationId string, partnerId null.String, webhookId string,
) error {
	webhook, err := usecase.convoyRepository.GetWebhook(ctx, webhookId)
	if err != nil {
		return models.NotFoundError
	}
	if err = usecase.enforceSecurity.CanModifyWebhook(ctx, webhook); err != nil {
		return err
	}

	err = usecase.convoyRepository.DeleteWebhook(ctx, webhook.Id)
	return errors.Wrap(err, "error deleting webhook")
}

func (usecase WebhooksUsecase) UpdateWebhook(
	ctx context.Context, organizationId string, partnerId null.String, webhookId string, input models.WebhookUpdate,
) (models.Webhook, error) {
	if err := input.Validate(); err != nil {
		return models.Webhook{}, err
	}

	webhook, err := usecase.convoyRepository.GetWebhook(ctx, webhookId)
	if err != nil {
		return models.Webhook{}, models.NotFoundError
	}
	if err = usecase.enforceSecurity.CanModifyWebhook(ctx, webhook); err != nil {
		return models.Webhook{}, err
	}

	updatedWebhook, err := usecase.convoyRepository.UpdateWebhook(ctx,
		models.MergeWebhookWithUpdate(webhook, input))
	return updatedWebhook, errors.Wrap(err, "error updating webhook")
}
