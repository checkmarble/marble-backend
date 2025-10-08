package billing

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	lago_repository "github.com/checkmarble/marble-backend/repositories/lago"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type BillingUsecaseInterface interface {
	SendEvent(ctx context.Context, orgId string, event models.BillingEvent) error
	CheckIfEnoughFundsInWallet(ctx context.Context, orgId string, code BillableMetric) (bool, string, error)
}

func NewBillingUsecase(
	isLagoConfigured bool,
	lagoRepository lago_repository.LagoRepository,
	transactionFactory executor_factory.TransactionFactory,
	enqueueSendBillingEventTask enqueueSendBillingEventTask,
) BillingUsecaseInterface {
	if isLagoConfigured {
		return NewLagoBillingUsecase(
			lagoRepository,
			transactionFactory,
			enqueueSendBillingEventTask,
		)
	}
	return NewDisabledBillingUsecase()
}
