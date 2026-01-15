package billing

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type NoOpBillingUsecase struct{}

func NewNoOpBillingUsecase() NoOpBillingUsecase {
	return NoOpBillingUsecase{}
}

func (u NoOpBillingUsecase) EnqueueBillingEventTask(ctx context.Context, event models.BillingEvent) error {
	return nil
}

func (u NoOpBillingUsecase) CheckIfEnoughFundsInWallet(ctx context.Context, orgId uuid.UUID, code BillableMetric) (bool, string, error) {
	return true, "Fake subscription ID", nil
}
