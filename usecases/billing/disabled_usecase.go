package billing

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
)

type DisabledBillingUsecase struct{}

func NewDisabledBillingUsecase() DisabledBillingUsecase {
	return DisabledBillingUsecase{}
}

func (u DisabledBillingUsecase) SendEvent(ctx context.Context, orgId string, event models.BillingEvent) error {
	return nil
}

func (u DisabledBillingUsecase) CheckIfEnoughFundsInWallet(ctx context.Context, orgId string, code BillableMetric) (bool, string, error) {
	return true, "Fake subscription ID", nil
}
