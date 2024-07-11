package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/guregu/null/v5"
	"github.com/pkg/errors"
)

type convoyWebhooksRepository interface {
	CreateWebhook(ctx context.Context, input models.WebhookCreate) error
}

type enforceSecurityWebhook interface {
	CanManageWebhook(ctx context.Context, organizationId string, partnerId null.String) error
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

func (usecase WebhooksUsecase) CreateWebhook(
	ctx context.Context,
	input models.WebhookCreate,
) error {
	err := usecase.enforceSecurity.CanManageWebhook(ctx, input.OrganizationId, input.PartnerId)
	if err != nil {
		return err
	}

	if err = input.Validate(); err != nil {
		return err
	}

	// TODO(webhook): generate secret
	input.Secret = "secret"

	err = usecase.convoyRepository.CreateWebhook(ctx, input)
	if err != nil {
		return errors.Wrap(err, "error creating webhook event")
	}

	return nil
}
