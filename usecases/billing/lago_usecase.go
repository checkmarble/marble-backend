package billing

import (
	"context"
	"errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type lagoRepository interface {
	GetWallets(ctx context.Context, orgId uuid.UUID) ([]models.Wallet, error)
	GetSubscriptions(ctx context.Context, orgId uuid.UUID) ([]models.Subscription, error)
	GetSubscription(ctx context.Context, subscriptionExternalId string) (models.Subscription, error)
	GetCustomerUsage(ctx context.Context, orgId uuid.UUID, subscriptionExternalId string) (models.CustomerUsage, error)
}

type billingEventTaskEnqueuer interface {
	EnqueueSendBillingEventTask(ctx context.Context, event models.BillingEvent) error
}

type LagoBillingUsecase struct {
	lagoRepository lagoRepository

	billingEventTaskEnqueuer billingEventTaskEnqueuer
}

func NewLagoBillingUsecase(
	lagoRepository lagoRepository,
	billingEventTaskEnqueuer billingEventTaskEnqueuer,
) LagoBillingUsecase {
	return LagoBillingUsecase{
		lagoRepository:           lagoRepository,
		billingEventTaskEnqueuer: billingEventTaskEnqueuer,
	}
}

// Send an event to Lago in async
func (u LagoBillingUsecase) EnqueueBillingEventTask(ctx context.Context, event models.BillingEvent) error {
	return u.billingEventTaskEnqueuer.EnqueueSendBillingEventTask(ctx, event)
}

// Check if there are enough funds in the wallet to cover the cost of the event
// Check if the wallet exists and if the balance is enough
// Suppose there is only one subscription for the event
func (u LagoBillingUsecase) CheckIfEnoughFundsInWallet(ctx context.Context, orgId uuid.UUID, code BillableMetric) (bool, string, error) {
	logger := utils.LoggerFromContext(ctx)

	wallets, err := u.lagoRepository.GetWallets(ctx, orgId)
	if err != nil {
		return false, "", err
	}
	if len(wallets) == 0 {
		logger.DebugContext(ctx, "no wallet found for the organization", "orgId", orgId)
		return false, "", nil
	}

	activeWallet, err := selectActiveWallet(wallets)
	if err != nil {
		logger.DebugContext(ctx, "no active wallet found", "orgId", orgId, "error", err)
		return false, "", nil
	}

	// Expect only one subscription for the event, in case of multiple subscriptions, we will use the first one
	subscriptions, err := u.getSubscriptionsForEvent(ctx, orgId, code)
	if err != nil {
		return false, "", err
	}
	if len(subscriptions) == 0 {
		logger.DebugContext(ctx, "no subscription found for the event", "orgId", orgId, "code", code)
		return false, "", nil
	} else if len(subscriptions) > 1 {
		logger.WarnContext(ctx, "multiple subscriptions found for the event", "orgId",
			orgId, "code", code, "subscriptions", subscriptions)
	}
	subscription := subscriptions[0]

	customerUsage, err := u.lagoRepository.GetCustomerUsage(ctx, orgId, subscription.ExternalId)
	if err != nil {
		return false, "", err
	}

	// For now, suppose there is only one wallet
	if activeWallet.BalanceCents <= customerUsage.TotalAmountCents {
		logger.DebugContext(ctx, "not enough funds in the wallet", "orgId", orgId, "code", code,
			"wallet", activeWallet.BalanceCents, "subscription",
			customerUsage.TotalAmountCents,
		)
		return false, "", nil
	}

	return true, subscription.ExternalId, nil
}

// Get all subscriptions of an organization that have a charge for the given billable metric
// Need to get first the list of subscriptions, then get the detailed subscription to get the charges
func (u LagoBillingUsecase) getSubscriptionsForEvent(ctx context.Context, orgId uuid.UUID, code BillableMetric) ([]models.Subscription, error) {
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

func selectActiveWallet(wallets []models.Wallet) (models.Wallet, error) {
	for _, wallet := range wallets {
		if wallet.Status == models.WalletStatusActive {
			return wallet, nil
		}
	}
	return models.Wallet{}, errors.New("no active wallet found")
}
