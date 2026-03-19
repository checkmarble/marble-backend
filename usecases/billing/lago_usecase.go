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
	GetEntitlements(ctx context.Context, subscriptionExternalId string) ([]models.BillingEntitlement, error)
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

// Check if there are enough funds in the wallet to cover the cost of the event for the given subscription.
func (u LagoBillingUsecase) CheckIfEnoughFundsInWallet(ctx context.Context, orgId uuid.UUID, subscriptionExternalId string, code BillableMetric) (bool, error) {
	logger := utils.LoggerFromContext(ctx)

	wallets, err := u.lagoRepository.GetWallets(ctx, orgId)
	if err != nil {
		return false, err
	}
	if len(wallets) == 0 {
		logger.DebugContext(ctx, "no wallet found for the organization", "orgId", orgId)
		return false, nil
	}

	activeWallet, err := selectActiveWallet(wallets)
	if err != nil {
		logger.DebugContext(ctx, "no active wallet found", "orgId", orgId, "error", err)
		return false, nil
	}

	customerUsage, err := u.lagoRepository.GetCustomerUsage(ctx, orgId, subscriptionExternalId)
	if err != nil {
		return false, err
	}

	var chargeUsageForCode *int
	for _, chargeUsage := range customerUsage.ChargesUsage {
		if chargeUsage.BillableMetric.Code == code.String() {
			chargeUsageForCode = &chargeUsage.AmountCents
			break
		}
	}
	if chargeUsageForCode == nil {
		logger.DebugContext(ctx, "no charge usage found for the billable metric in customer usage",
			"orgId", orgId, "code", code, "customerUsage", customerUsage)
		return false, errors.New("no charge usage found for the billable metric in customer usage")
	}
	logger.InfoContext(ctx, "customer usage for the event", "orgId", orgId, "code", code, "usage", *chargeUsageForCode)

	if activeWallet.BalanceCents <= *chargeUsageForCode {
		logger.DebugContext(ctx, "not enough funds in the wallet",
			"orgId", orgId,
			"code", code,
			"wallet_funds", activeWallet.BalanceCents,
			"usage", *chargeUsageForCode,
		)
		return false, nil
	}

	return true, nil
}

// GetSubscriptionsForEvent returns all subscriptions of an organization that have a charge
// for the given billable metric. It fetches the list of subscriptions and then retrieves
// the detailed subscription to inspect charges.
func (u LagoBillingUsecase) GetSubscriptionsForEvent(ctx context.Context, orgId uuid.UUID, code BillableMetric) ([]models.Subscription, error) {
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

// CheckEntitlement checks whether the given subscription has a specific billing entitlement.
func (u LagoBillingUsecase) CheckEntitlement(
	ctx context.Context,
	subscriptionExternalId string,
	entitlementCode BillingEntitlementCode,
) (bool, error) {
	logger := utils.LoggerFromContext(ctx)

	entitlements, err := u.lagoRepository.GetEntitlements(ctx, subscriptionExternalId)
	if err != nil {
		return false, err
	}
	for _, entitlement := range entitlements {
		if entitlement.Code == entitlementCode.String() {
			return true, nil
		}
	}

	logger.DebugContext(ctx, "entitlement not found for the subscription",
		"entitlementCode", entitlementCode)
	return false, nil
}
