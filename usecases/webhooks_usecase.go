package usecases

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/guregu/null/v5"
	"github.com/pkg/errors"
)

type convoyWebhooksRepository interface {
	ListWebhooks(ctx context.Context, organizationId string, partnerId null.String) ([]models.Webhook, error)
	RegisterWebhook(ctx context.Context, input models.WebhookRegister) error
	DeleteWebhook(ctx context.Context, webhookId string) error
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

func (usecase WebhooksUsecase) ListWebhooks(ctx context.Context, organizationId string, partnerId null.String) ([]models.Webhook, error) {
	err := usecase.enforceSecurity.CanManageWebhook(ctx, organizationId, partnerId)
	if err != nil {
		return nil, err
	}

	webhooks, err := usecase.convoyRepository.ListWebhooks(ctx, organizationId, partnerId)
	if err != nil {
		return nil, errors.Wrap(err, "error listing webhooks")
	}

	return webhooks, nil
}

func (usecase WebhooksUsecase) RegisterWebhook(
	ctx context.Context,
	input models.WebhookRegister,
) error {
	err := usecase.enforceSecurity.CanManageWebhook(ctx, input.OrganizationId, input.PartnerId)
	if err != nil {
		return err
	}

	if err = input.Validate(); err != nil {
		return err
	}

	input.Secret = generateSecret()

	err = usecase.convoyRepository.RegisterWebhook(ctx, input)
	if err != nil {
		return errors.Wrap(err, "error registering webhook")
	}

	return nil
}

func generateSecret() string {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		panic(fmt.Errorf("generateSecret: %w", err))
	}
	return hex.EncodeToString(key)
}

func (usecase WebhooksUsecase) DeleteWebhook(
	ctx context.Context, organizationId string, partnerId null.String, webhookId string,
) error {
	err := usecase.enforceSecurity.CanManageWebhook(ctx, organizationId, partnerId)
	if err != nil {
		return err
	}

	return usecase.convoyRepository.DeleteWebhook(ctx, webhookId)
}
