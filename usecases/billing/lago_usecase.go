package billing

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
)

type lagoRepository interface {
	GetWallet(ctx context.Context, orgId string) ([]models.Wallet, error)
	GetSubscriptions(ctx context.Context, orgId string) ([]models.Subscription, error)
	GetSubscription(ctx context.Context, subscriptionExternalId string) (models.Subscription, error)
	GetCustomerUsage(ctx context.Context, orgId string, subscriptionExternalId string) (models.CustomerUsage, error)
}

type enqueueSendBillingEventTask interface {
	EnqueueSendBillingEventTask(ctx context.Context, tx repositories.Transaction, orgId string, event models.BillingEvent) error
}

type LagoBillingUsecase struct {
	lagoRepository lagoRepository

	enqueueSendBillingEventTask enqueueSendBillingEventTask
}

func NewLagoBillingUsecase(
	lagoRepository lagoRepository,
	enqueueSendBillingEventTask enqueueSendBillingEventTask,
) LagoBillingUsecase {
	return LagoBillingUsecase{
		lagoRepository:              lagoRepository,
		enqueueSendBillingEventTask: enqueueSendBillingEventTask,
	}
}

// Send an event to Lago in async
func (u LagoBillingUsecase) SendEventAsync(ctx context.Context, tx repositories.Transaction, orgId string, event models.BillingEvent) error {
	return u.enqueueSendBillingEventTask.EnqueueSendBillingEventTask(ctx, tx, orgId, event)
}

// Check if there are enough funds in the wallet to cover the cost of the event
// Check if the wallet exists and if the balance is enough
// Suppose there is only one subscription for the event
func (u LagoBillingUsecase) CheckIfEnoughFundsInWallet(ctx context.Context, orgId string, code BillableMetric) (bool, string, error) {
	logger := utils.LoggerFromContext(ctx)

	wallet, err := u.lagoRepository.GetWallet(ctx, orgId)
	if err != nil {
		logger.Error("failed to get wallet", "orgId", orgId, "error", err)
		return false, "", err
	}
	if len(wallet) == 0 {
		logger.DebugContext(ctx, "no wallet found for the organization", "orgId", orgId)
		return false, "", nil
	}

	// Expect only one subscription for the event, in case of multiple subscriptions, we will use the first one
	subscriptions, err := u.getSubscriptionsForEvent(ctx, orgId, code)
	if err != nil {
		return false, "", err
	}
	if len(subscriptions) == 0 {
		logger.Debug("no subscription found for the event", "orgId", orgId, "code", code)
		return false, "", nil
	} else if len(subscriptions) > 1 {
		// Should we raise an error in this case?
		logger.Warn("multiple subscriptions found for the event", "orgId", orgId, "code", code, "subscriptions", subscriptions)
	}
	subscription := subscriptions[0]

	customerUsage, err := u.lagoRepository.GetCustomerUsage(ctx, orgId, subscription.ExternalId)
	if err != nil {
		logger.Error("failed to get customer usage", "orgId", orgId, "error", err)
		return false, "", err
	}

	// For now, suppose there is only one wallet
	if wallet[0].BalanceCents <= customerUsage.TotalAmountCents {
		logger.Debug("not enough funds in the wallet", "orgId", orgId, "code", code,
			"wallet", wallet[0].BalanceCents, "subscription",
			customerUsage.TotalAmountCents,
		)
		return false, "", nil
	}

	return true, subscription.ExternalId, nil
}

// Get all subscriptions of an organization that have a charge for the given billable metric
// Need to get first the list of subscriptions, then get the detailed subscription to get the charges
func (u LagoBillingUsecase) getSubscriptionsForEvent(ctx context.Context, orgId string, code BillableMetric) ([]models.Subscription, error) {
	subscriptionsForEvent := make([]models.Subscription, 0)
	subscriptions, err := u.lagoRepository.GetSubscriptions(ctx, orgId)
	if err != nil {
		return nil, err
	}
	for _, subscription := range subscriptions {
		subscriptionDetailed, err := u.lagoRepository.GetSubscription(ctx, subscription.ExternalId)
		if err != nil {
			return nil, err
		}
		for _, charge := range subscriptionDetailed.Plan.Charges {
			if charge.BillableMetricCode == code.String() {
				subscriptionsForEvent = append(subscriptionsForEvent, subscription)
				break
			}
		}
	}
	return subscriptionsForEvent, nil
}
