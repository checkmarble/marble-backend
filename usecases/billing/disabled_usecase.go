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

func (u NoOpBillingUsecase) GetSubscriptionsForEvent(ctx context.Context, orgId uuid.UUID, code BillableMetric) ([]models.Subscription, error) {
	return []models.Subscription{}, nil
}

func (u NoOpBillingUsecase) CheckIfEnoughFundsInWallet(
	ctx context.Context,
	orgId uuid.UUID,
	subscriptionExternalId string,
	code BillableMetric,
) (bool, error) {
	return true, nil
}

func (u NoOpBillingUsecase) CheckEntitlement(ctx context.Context, subscriptionExternalId string, entitlementCode BillingEntitlementCode) (bool, error) {
	return true, nil
}
