package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/mock"
)

type LagoRepository struct {
	mock.Mock
}

func (r *LagoRepository) GetWallets(ctx context.Context, orgId string) ([]models.Wallet, error) {
	args := r.Called(ctx, orgId)
	return args.Get(0).([]models.Wallet), args.Error(1)
}

func (r *LagoRepository) GetSubscriptions(ctx context.Context, orgId string) ([]models.Subscription, error) {
	args := r.Called(ctx, orgId)
	return args.Get(0).([]models.Subscription), args.Error(1)
}

func (r *LagoRepository) GetSubscription(ctx context.Context, subscriptionExternalId string) (models.Subscription, error) {
	args := r.Called(ctx, subscriptionExternalId)
	return args.Get(0).(models.Subscription), args.Error(1)
}

func (r *LagoRepository) GetCustomerUsage(
	ctx context.Context,
	orgId string,
	subscriptionExternalId string,
) (models.CustomerUsage, error) {
	args := r.Called(ctx, orgId, subscriptionExternalId)
	return args.Get(0).(models.CustomerUsage), args.Error(1)
}

func (r *LagoRepository) SendEvent(ctx context.Context, event models.BillingEvent) error {
	args := r.Called(ctx, event)
	return args.Error(0)
}

func (r *LagoRepository) SendEvents(ctx context.Context, events []models.BillingEvent) error {
	args := r.Called(ctx, events)
	return args.Error(0)
}
