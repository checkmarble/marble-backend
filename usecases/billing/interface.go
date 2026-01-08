package billing

import (
	"context"
	"errors"

	"github.com/checkmarble/marble-backend/models"
	lago_repository "github.com/checkmarble/marble-backend/repositories/lago"
	"github.com/google/uuid"
)

var ErrInsufficientFunds = errors.New("insufficient funds in wallet")

type BillingUsecase interface {
	EnqueueBillingEventTask(ctx context.Context, event models.BillingEvent) error
	CheckIfEnoughFundsInWallet(ctx context.Context, orgId uuid.UUID, code BillableMetric) (bool, string, error)
}

// Factory function to create the appropriate billing usecase
func NewBillingUsecase(
	isLagoConfigured bool,
	lagoRepository lago_repository.LagoRepository,
	enqueueSendBillingEventTask billingEventTaskEnqueuer,
) BillingUsecase {
	if isLagoConfigured {
		return NewLagoBillingUsecase(
			lagoRepository,
			enqueueSendBillingEventTask,
		)
	}
	return NewNoOpBillingUsecase()
}
