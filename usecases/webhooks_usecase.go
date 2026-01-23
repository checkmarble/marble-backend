package usecases

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/guregu/null/v5"
)

type webhooksRepository interface {
	GetWebhook(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.Webhook, error)
	ListWebhooks(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, partnerId *uuid.UUID) ([]models.Webhook, error)
	CreateWebhook(ctx context.Context, exec repositories.Executor, input models.WebhookRegister, orgId uuid.UUID, partnerId *uuid.UUID, secretValue string) (models.Webhook, error)
	UpdateWebhook(ctx context.Context, exec repositories.Executor, webhook models.Webhook) (models.Webhook, error)
	DeleteWebhook(ctx context.Context, exec repositories.Executor, id uuid.UUID) error
}

type enforceSecurityWebhook interface {
	CanCreateWebhook(ctx context.Context, organizationId uuid.UUID, partnerId null.String) error
	CanReadWebhook(ctx context.Context, webhook models.Webhook) error
	CanModifyWebhook(ctx context.Context, webhook models.Webhook) error
}

type WebhooksUsecase struct {
	enforceSecurity    enforceSecurityWebhook
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	webhookRepository  webhooksRepository
}

func NewWebhooksUsecase(
	enforceSecurity enforceSecurityWebhook,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	webhookRepository webhooksRepository,
) WebhooksUsecase {
	return WebhooksUsecase{
		enforceSecurity:    enforceSecurity,
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		webhookRepository:  webhookRepository,
	}
}

func (usecase WebhooksUsecase) ListWebhooks(ctx context.Context, organizationId uuid.UUID, partnerId null.String) ([]models.Webhook, error) {
	exec := usecase.executorFactory.NewExecutor()

	var partnerIdPtr *uuid.UUID
	if partnerId.Valid {
		id, err := uuid.Parse(partnerId.String)
		if err == nil {
			partnerIdPtr = &id
		}
	}

	webhooks, err := usecase.webhookRepository.ListWebhooks(ctx, exec, organizationId, partnerIdPtr)
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
	organizationId uuid.UUID,
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

	// Validate endpoint reachability
	timeout := 10 * time.Second
	if input.HttpTimeout != nil {
		timeout = time.Duration(*input.HttpTimeout) * time.Second
	}
	if err := ValidateWebhookEndpoint(ctx, input.Url, timeout); err != nil {
		return models.Webhook{}, err
	}

	// Generate a secret for the webhook
	secretValue, err := models.GenerateWebhookSecret()
	if err != nil {
		return models.Webhook{}, errors.Wrap(err, "error generating webhook secret")
	}

	var partnerIdPtr *uuid.UUID
	if partnerId.Valid {
		id, err := uuid.Parse(partnerId.String)
		if err == nil {
			partnerIdPtr = &id
		}
	}

	exec := usecase.executorFactory.NewExecutor()

	webhook, err := usecase.webhookRepository.CreateWebhook(ctx, exec, input, organizationId, partnerIdPtr, secretValue)
	return webhook, errors.Wrap(err, "error registering webhook")
}

func (usecase WebhooksUsecase) GetWebhook(
	ctx context.Context, organizationId uuid.UUID, partnerId null.String, webhookId string,
) (models.Webhook, error) {
	exec := usecase.executorFactory.NewExecutor()

	id, err := uuid.Parse(webhookId)
	if err != nil {
		return models.Webhook{}, models.BadParameterError
	}

	webhook, err := usecase.webhookRepository.GetWebhook(ctx, exec, id)
	if err != nil {
		return models.Webhook{}, models.NotFoundError
	}
	if err = usecase.enforceSecurity.CanReadWebhook(ctx, webhook); err != nil {
		return models.Webhook{}, err
	}
	return webhook, nil
}

func (usecase WebhooksUsecase) DeleteWebhook(
	ctx context.Context, organizationId uuid.UUID, partnerId null.String, webhookId string,
) error {
	exec := usecase.executorFactory.NewExecutor()

	id, err := uuid.Parse(webhookId)
	if err != nil {
		return models.BadParameterError
	}

	webhook, err := usecase.webhookRepository.GetWebhook(ctx, exec, id)
	if err != nil {
		return models.NotFoundError
	}
	if err = usecase.enforceSecurity.CanModifyWebhook(ctx, webhook); err != nil {
		return err
	}

	err = usecase.webhookRepository.DeleteWebhook(ctx, exec, webhook.Id)
	return errors.Wrap(err, "error deleting webhook")
}

func (usecase WebhooksUsecase) UpdateWebhook(
	ctx context.Context, organizationId uuid.UUID, partnerId null.String, webhookId string, input models.WebhookUpdate,
) (models.Webhook, error) {
	if err := input.Validate(); err != nil {
		return models.Webhook{}, err
	}

	exec := usecase.executorFactory.NewExecutor()

	id, err := uuid.Parse(webhookId)
	if err != nil {
		return models.Webhook{}, models.BadParameterError
	}

	webhook, err := usecase.webhookRepository.GetWebhook(ctx, exec, id)
	if err != nil {
		return models.Webhook{}, models.NotFoundError
	}
	if err = usecase.enforceSecurity.CanModifyWebhook(ctx, webhook); err != nil {
		return models.Webhook{}, err
	}

	// Validate new endpoint if URL changed
	if input.Url != nil && *input.Url != webhook.Url {
		timeout := 10 * time.Second
		if input.HttpTimeout != nil {
			timeout = time.Duration(*input.HttpTimeout) * time.Second
		} else if webhook.HttpTimeout != nil {
			timeout = time.Duration(*webhook.HttpTimeout) * time.Second
		}
		if err := ValidateWebhookEndpoint(ctx, *input.Url, timeout); err != nil {
			return models.Webhook{}, err
		}
	}

	updatedWebhook, err := usecase.webhookRepository.UpdateWebhook(ctx, exec,
		models.MergeWebhookWithUpdate(webhook, input))
	return updatedWebhook, errors.Wrap(err, "error updating webhook")
}
