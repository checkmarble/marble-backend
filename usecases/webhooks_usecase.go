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
	RegisterWebhook(ctx context.Context, input models.WebhookRegister) error
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
