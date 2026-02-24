package billing

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
)

type LagoBillingUsecaseTestSuite struct {
	suite.Suite
	lagoRepository           *mocks.LagoRepository
	billingEventTaskEnqueuer *mocks.TaskQueueRepository

	ctx   context.Context
	orgId uuid.UUID
}

func (suite *LagoBillingUsecaseTestSuite) SetupTest() {
	suite.lagoRepository = new(mocks.LagoRepository)
	suite.billingEventTaskEnqueuer = new(mocks.TaskQueueRepository)

	suite.ctx = context.Background()
	suite.orgId = uuid.MustParse("12345678-1234-5678-9012-345678901234")
}

func (suite *LagoBillingUsecaseTestSuite) makeUsecase() LagoBillingUsecase {
	return NewLagoBillingUsecase(
		suite.lagoRepository,
		suite.billingEventTaskEnqueuer,
	)
}

func (suite *LagoBillingUsecaseTestSuite) AssertExpectations() {
	t := suite.T()
	suite.lagoRepository.AssertExpectations(t)
	suite.billingEventTaskEnqueuer.AssertExpectations(t)
}

func TestLagoBillingUsecase(t *testing.T) {
	suite.Run(t, new(LagoBillingUsecaseTestSuite))
}

// Test getSubscriptionsForEvent - return the right list with subscription with the right code match
func (suite *LagoBillingUsecaseTestSuite) Test_getSubscriptionsForEvent_ReturnsMatchingSubscriptions() {
	usecase := suite.makeUsecase()

	// Create subscriptions with different billable metric codes
	subscriptions := []models.Subscription{
		{
			Id:         "sub-1",
			ExternalId: "ext-sub-1",
			Status:     "active",
		},
		{
			Id:         "sub-2",
			ExternalId: "ext-sub-2",
			Status:     "active",
		},
		{
			Id:         "sub-3",
			ExternalId: "ext-sub-3",
			Status:     "active",
		},
	}

	// Detailed subscriptions - only sub-1 and sub-3 have the AI_CASE_REVIEW code
	detailedSub1 := models.Subscription{
		Id:         "sub-1",
		ExternalId: "ext-sub-1",
		Status:     "active",
		Plan: models.Plan{
			Id:   "plan-1",
			Name: "Basic Plan",
			Charges: []models.Charge{
				{
					Id:                 "charge-1",
					BillableMetricCode: AI_CASE_REVIEW.String(),
				},
				{
					Id:                 "charge-2",
					BillableMetricCode: UNKNOWN.String(),
				},
			},
		},
	}

	detailedSub2 := models.Subscription{
		Id:         "sub-2",
		ExternalId: "ext-sub-2",
		Status:     "active",
		Plan: models.Plan{
			Id:   "plan-2",
			Name: "Standard Plan",
			Charges: []models.Charge{
				{
					Id:                 "charge-3",
					BillableMetricCode: UNKNOWN.String(),
				},
			},
		},
	}

	detailedSub3 := models.Subscription{
		Id:         "sub-3",
		ExternalId: "ext-sub-3",
		Status:     "active",
		Plan: models.Plan{
			Id:   "plan-3",
			Name: "Premium Plan",
			Charges: []models.Charge{
				{
					Id:                 "charge-4",
					BillableMetricCode: AI_CASE_REVIEW.String(),
				},
			},
		},
	}

	suite.lagoRepository.On("GetSubscriptions", suite.ctx, suite.orgId).Return(subscriptions, nil)
	suite.lagoRepository.On("GetSubscription", suite.ctx, "ext-sub-1").Return(detailedSub1, nil)
	suite.lagoRepository.On("GetSubscription", suite.ctx, "ext-sub-2").Return(detailedSub2, nil)
	suite.lagoRepository.On("GetSubscription", suite.ctx, "ext-sub-3").Return(detailedSub3, nil)

	result, err := usecase.getSubscriptionsForEvent(suite.ctx, suite.orgId, AI_CASE_REVIEW)

	suite.NoError(err)
	suite.Len(result, 2, "should return 2 subscriptions with AI_CASE_REVIEW code")
	suite.Equal("sub-1", result[0].Id)
	suite.Equal("sub-3", result[1].Id)
	suite.AssertExpectations()
}

func (suite *LagoBillingUsecaseTestSuite) Test_getSubscriptionsForEvent_NoMatchingSubscriptions() {
	usecase := suite.makeUsecase()

	subscriptions := []models.Subscription{
		{
			Id:         "sub-1",
			ExternalId: "ext-sub-1",
			Status:     "active",
		},
	}

	detailedSub1 := models.Subscription{
		Id:         "sub-1",
		ExternalId: "ext-sub-1",
		Status:     "active",
		Plan: models.Plan{
			Id:   "plan-1",
			Name: "Basic Plan",
			Charges: []models.Charge{
				{
					Id:                 "charge-1",
					BillableMetricCode: UNKNOWN.String(),
				},
			},
		},
	}

	suite.lagoRepository.On("GetSubscriptions", suite.ctx, suite.orgId).Return(subscriptions, nil)
	suite.lagoRepository.On("GetSubscription", suite.ctx, "ext-sub-1").Return(detailedSub1, nil)

	result, err := usecase.getSubscriptionsForEvent(suite.ctx, suite.orgId, AI_CASE_REVIEW)

	suite.NoError(err)
	suite.Empty(result, "should return empty list when no matching subscriptions")
	suite.AssertExpectations()
}

// Test CheckIfEnoughFundsInWallet - Case of no wallet
func (suite *LagoBillingUsecaseTestSuite) Test_CheckIfEnoughFundsInWallet_NoWallet() {
	usecase := suite.makeUsecase()

	// Return empty wallet list
	suite.lagoRepository.On("GetWallets", suite.ctx, suite.orgId).Return([]models.Wallet{}, nil)

	hasEnoughFunds, subscriptionId, err :=
		usecase.CheckIfEnoughFundsInWallet(suite.ctx, suite.orgId, AI_CASE_REVIEW)

	suite.NoError(err)
	suite.False(hasEnoughFunds, "should return false when no wallet exists")
	suite.Empty(subscriptionId, "should return empty subscription ID")
	suite.AssertExpectations()
}

// Test CheckIfEnoughFundsInWallet - Case of no active wallet
func (suite *LagoBillingUsecaseTestSuite) Test_CheckIfEnoughFundsInWallet_NoActiveWallet() {
	usecase := suite.makeUsecase()

	wallets := []models.Wallet{
		{
			Id:           "wallet-1",
			Status:       "terminated",
			BalanceCents: 10000,
		},
	}

	suite.lagoRepository.On("GetWallets", suite.ctx, suite.orgId).Return(wallets, nil)

	hasEnoughFunds, subscriptionId, err :=
		usecase.CheckIfEnoughFundsInWallet(suite.ctx, suite.orgId, AI_CASE_REVIEW)

	suite.NoError(err)
	suite.False(hasEnoughFunds, "should return false when no active wallet exists")
	suite.Empty(subscriptionId, "should return empty subscription ID")
	suite.AssertExpectations()
}

// Test CheckIfEnoughFundsInWallet - Case with wallet but no right subscription
func (suite *LagoBillingUsecaseTestSuite) Test_CheckIfEnoughFundsInWallet_NoMatchingSubscription() {
	usecase := suite.makeUsecase()

	wallet := []models.Wallet{
		{
			Id:           "wallet-1",
			Status:       "active",
			BalanceCents: 10000,
		},
	}

	subscriptions := []models.Subscription{
		{
			Id:         "sub-1",
			ExternalId: "ext-sub-1",
			Status:     "active",
		},
	}

	detailedSub1 := models.Subscription{
		Id:         "sub-1",
		ExternalId: "ext-sub-1",
		Status:     "active",
		Plan: models.Plan{
			Id:   "plan-1",
			Name: "Basic Plan",
			Charges: []models.Charge{
				{
					Id:                 "charge-1",
					BillableMetricCode: UNKNOWN.String(), // Different code
				},
			},
		},
	}

	suite.lagoRepository.On("GetWallets", suite.ctx, suite.orgId).Return(wallet, nil)
	suite.lagoRepository.On("GetSubscriptions", suite.ctx, suite.orgId).Return(subscriptions, nil)
	suite.lagoRepository.On("GetSubscription", suite.ctx, "ext-sub-1").Return(detailedSub1, nil)

	hasEnoughFunds, subscriptionId, err :=
		usecase.CheckIfEnoughFundsInWallet(suite.ctx, suite.orgId, AI_CASE_REVIEW)

	suite.NoError(err)
	suite.False(hasEnoughFunds, "should return false when no matching subscription")
	suite.Empty(subscriptionId, "should return empty subscription ID")
	suite.AssertExpectations()
}

// Test CheckIfEnoughFundsInWallet - Case with several subscriptions and take the first one
func (suite *LagoBillingUsecaseTestSuite) Test_CheckIfEnoughFundsInWallet_MultipleSubscriptions_TakesFirst() {
	usecase := suite.makeUsecase()

	wallet := []models.Wallet{
		{
			Id:           "wallet-1",
			Status:       "active",
			BalanceCents: 10000,
		},
	}

	subscriptions := []models.Subscription{
		{
			Id:         "sub-1",
			ExternalId: "ext-sub-1",
			Status:     "active",
		},
		{
			Id:         "sub-2",
			ExternalId: "ext-sub-2",
			Status:     "active",
		},
	}

	// Both subscriptions have AI_CASE_REVIEW charge
	detailedSub1 := models.Subscription{
		Id:         "sub-1",
		ExternalId: "ext-sub-1",
		Status:     "active",
		Plan: models.Plan{
			Id:   "plan-1",
			Name: "Basic Plan",
			Charges: []models.Charge{
				{
					Id:                 "charge-1",
					BillableMetricCode: AI_CASE_REVIEW.String(),
				},
			},
		},
	}

	detailedSub2 := models.Subscription{
		Id:         "sub-2",
		ExternalId: "ext-sub-2",
		Status:     "active",
		Plan: models.Plan{
			Id:   "plan-2",
			Name: "Premium Plan",
			Charges: []models.Charge{
				{
					Id:                 "charge-2",
					BillableMetricCode: AI_CASE_REVIEW.String(),
				},
			},
		},
	}

	customerUsage := models.CustomerUsage{
		TotalAmountCents: 5000,
		ChargesUsage: []models.ChargeUsage{
			{
				AmountCents: 5000,
				BillableMetric: models.BillableMetricInUsage{
					Code: AI_CASE_REVIEW.String(),
				},
			},
		},
	}

	suite.lagoRepository.On("GetWallets", suite.ctx, suite.orgId).Return(wallet, nil)
	suite.lagoRepository.On("GetSubscriptions", suite.ctx, suite.orgId).Return(subscriptions, nil)
	suite.lagoRepository.On("GetSubscription", suite.ctx, "ext-sub-1").Return(detailedSub1, nil)
	suite.lagoRepository.On("GetSubscription", suite.ctx, "ext-sub-2").Return(detailedSub2, nil)
	// Should only get customer usage for the first subscription
	suite.lagoRepository.On("GetCustomerUsage", suite.ctx, suite.orgId, "ext-sub-1").Return(customerUsage, nil)

	hasEnoughFunds, subscriptionId, err :=
		usecase.CheckIfEnoughFundsInWallet(suite.ctx, suite.orgId, AI_CASE_REVIEW)

	suite.NoError(err)
	suite.True(hasEnoughFunds, "should return true when wallet has enough funds")
	suite.Equal("ext-sub-1", subscriptionId, "should return the first subscription ID")
	suite.AssertExpectations()
}

// Test CheckIfEnoughFundsInWallet - Case with wallet and enough funds
func (suite *LagoBillingUsecaseTestSuite) Test_CheckIfEnoughFundsInWallet_EnoughFunds() {
	usecase := suite.makeUsecase()

	wallet := []models.Wallet{
		{
			Id:           "wallet-1",
			Status:       "active",
			BalanceCents: 10000, // 100.00 in currency
		},
	}

	subscriptions := []models.Subscription{
		{
			Id:         "sub-1",
			ExternalId: "ext-sub-1",
			Status:     "active",
		},
	}

	detailedSub1 := models.Subscription{
		Id:         "sub-1",
		ExternalId: "ext-sub-1",
		Status:     "active",
		Plan: models.Plan{
			Id:   "plan-1",
			Name: "Basic Plan",
			Charges: []models.Charge{
				{
					Id:                 "charge-1",
					BillableMetricCode: AI_CASE_REVIEW.String(),
				},
			},
		},
	}

	customerUsage := models.CustomerUsage{
		TotalAmountCents: 5000, // 50.00 in currency - less than wallet balance
		ChargesUsage: []models.ChargeUsage{
			{
				AmountCents: 5000,
				BillableMetric: models.BillableMetricInUsage{
					Code: AI_CASE_REVIEW.String(),
				},
			},
		},
	}

	suite.lagoRepository.On("GetWallets", suite.ctx, suite.orgId).Return(wallet, nil)
	suite.lagoRepository.On("GetSubscriptions", suite.ctx, suite.orgId).Return(subscriptions, nil)
	suite.lagoRepository.On("GetSubscription", suite.ctx, "ext-sub-1").Return(detailedSub1, nil)
	suite.lagoRepository.On("GetCustomerUsage", suite.ctx, suite.orgId, "ext-sub-1").Return(customerUsage, nil)

	hasEnoughFunds, subscriptionId, err :=
		usecase.CheckIfEnoughFundsInWallet(suite.ctx, suite.orgId, AI_CASE_REVIEW)

	suite.NoError(err)
	suite.True(hasEnoughFunds, "should return true when wallet balance is greater than usage")
	suite.Equal("ext-sub-1", subscriptionId, "should return the subscription external ID")
	suite.AssertExpectations()
}

// Test CheckIfEnoughFundsInWallet - Case with wallet but not enough funds
func (suite *LagoBillingUsecaseTestSuite) Test_CheckIfEnoughFundsInWallet_NotEnoughFunds() {
	usecase := suite.makeUsecase()

	wallet := []models.Wallet{
		{
			Id:           "wallet-1",
			Status:       "active",
			BalanceCents: 5000, // 50.00 in currency
		},
	}

	subscriptions := []models.Subscription{
		{
			Id:         "sub-1",
			ExternalId: "ext-sub-1",
			Status:     "active",
		},
	}

	detailedSub1 := models.Subscription{
		Id:         "sub-1",
		ExternalId: "ext-sub-1",
		Status:     "active",
		Plan: models.Plan{
			Id:   "plan-1",
			Name: "Basic Plan",
			Charges: []models.Charge{
				{
					Id:                 "charge-1",
					BillableMetricCode: AI_CASE_REVIEW.String(),
				},
			},
		},
	}

	customerUsage := models.CustomerUsage{
		TotalAmountCents: 10000, // 100.00 in currency - more than wallet balance
		ChargesUsage: []models.ChargeUsage{
			{
				AmountCents: 10000,
				BillableMetric: models.BillableMetricInUsage{
					Code: AI_CASE_REVIEW.String(),
				},
			},
		},
	}

	suite.lagoRepository.On("GetWallets", suite.ctx, suite.orgId).Return(wallet, nil)
	suite.lagoRepository.On("GetSubscriptions", suite.ctx, suite.orgId).Return(subscriptions, nil)
	suite.lagoRepository.On("GetSubscription", suite.ctx, "ext-sub-1").Return(detailedSub1, nil)
	suite.lagoRepository.On("GetCustomerUsage", suite.ctx, suite.orgId, "ext-sub-1").Return(customerUsage, nil)

	hasEnoughFunds, subscriptionId, err :=
		usecase.CheckIfEnoughFundsInWallet(suite.ctx, suite.orgId, AI_CASE_REVIEW)

	suite.NoError(err)
	suite.False(hasEnoughFunds, "should return false when usage exceeds wallet balance")
	suite.Empty(subscriptionId, "should return empty subscription ID when not enough funds")
	suite.AssertExpectations()
}

// Test CheckIfEnoughFundsInWallet - Case with wallet but not enough funds (equal case)
func (suite *LagoBillingUsecaseTestSuite) Test_CheckIfEnoughFundsInWallet_ExactlyEqualFunds() {
	usecase := suite.makeUsecase()

	wallet := []models.Wallet{
		{
			Id:           "wallet-1",
			Status:       "active",
			BalanceCents: 5000,
		},
	}

	subscriptions := []models.Subscription{
		{
			Id:         "sub-1",
			ExternalId: "ext-sub-1",
			Status:     "active",
		},
	}

	detailedSub1 := models.Subscription{
		Id:         "sub-1",
		ExternalId: "ext-sub-1",
		Status:     "active",
		Plan: models.Plan{
			Id:   "plan-1",
			Name: "Basic Plan",
			Charges: []models.Charge{
				{
					Id:                 "charge-1",
					BillableMetricCode: AI_CASE_REVIEW.String(),
				},
			},
		},
	}

	customerUsage := models.CustomerUsage{
		TotalAmountCents: 5000, // Exactly equal to wallet balance
		ChargesUsage: []models.ChargeUsage{
			{
				AmountCents: 5000,
				BillableMetric: models.BillableMetricInUsage{
					Code: AI_CASE_REVIEW.String(),
				},
			},
		},
	}

	suite.lagoRepository.On("GetWallets", suite.ctx, suite.orgId).Return(wallet, nil)
	suite.lagoRepository.On("GetSubscriptions", suite.ctx, suite.orgId).Return(subscriptions, nil)
	suite.lagoRepository.On("GetSubscription", suite.ctx, "ext-sub-1").Return(detailedSub1, nil)
	suite.lagoRepository.On("GetCustomerUsage", suite.ctx, suite.orgId, "ext-sub-1").Return(customerUsage, nil)

	hasEnoughFunds, subscriptionId, err :=
		usecase.CheckIfEnoughFundsInWallet(suite.ctx, suite.orgId, AI_CASE_REVIEW)

	suite.NoError(err)
	suite.False(hasEnoughFunds, "should return false when usage equals wallet balance (not strictly greater)")
	suite.Empty(subscriptionId, "should return empty subscription ID")
	suite.AssertExpectations()
}

// Test CheckIfEnoughFundsInWallet - Case where the metric code is not in ChargesUsage: the subscription is not correctly set
func (suite *LagoBillingUsecaseTestSuite) Test_CheckIfEnoughFundsInWallet_MetricCodeNotInChargesUsage() {
	usecase := suite.makeUsecase()

	wallet := []models.Wallet{
		{
			Id:           "wallet-1",
			Status:       "active",
			BalanceCents: 5000,
		},
	}

	subscriptions := []models.Subscription{
		{
			Id:         "sub-1",
			ExternalId: "ext-sub-1",
			Status:     "active",
		},
	}

	detailedSub1 := models.Subscription{
		Id:         "sub-1",
		ExternalId: "ext-sub-1",
		Status:     "active",
		Plan: models.Plan{
			Id:   "plan-1",
			Name: "Basic Plan",
			Charges: []models.Charge{
				{
					Id:                 "charge-1",
					BillableMetricCode: AI_CASE_REVIEW.String(),
				},
			},
		},
	}

	// CustomerUsage does not contain a charge for AI_CASE_REVIEW, meaning the subscription is not correctly configured
	customerUsage := models.CustomerUsage{
		TotalAmountCents: 10000,
		ChargesUsage: []models.ChargeUsage{
			{
				AmountCents: 10000,
				BillableMetric: models.BillableMetricInUsage{
					Code: UNKNOWN.String(),
				},
			},
		},
	}

	suite.lagoRepository.On("GetWallets", suite.ctx, suite.orgId).Return(wallet, nil)
	suite.lagoRepository.On("GetSubscriptions", suite.ctx, suite.orgId).Return(subscriptions, nil)
	suite.lagoRepository.On("GetSubscription", suite.ctx, "ext-sub-1").Return(detailedSub1, nil)
	suite.lagoRepository.On("GetCustomerUsage", suite.ctx, suite.orgId, "ext-sub-1").Return(customerUsage, nil)

	hasEnoughFunds, subscriptionId, err :=
		usecase.CheckIfEnoughFundsInWallet(suite.ctx, suite.orgId, AI_CASE_REVIEW)

	suite.Error(err, "should return an error when metric code is not found in charges usage")
	suite.False(hasEnoughFunds)
	suite.Empty(subscriptionId)
	suite.AssertExpectations()
}

// Test CheckIfEnoughFundsInWallet - Total of two charges exceeds wallet, but AI_CASE_REVIEW alone is below
func (suite *LagoBillingUsecaseTestSuite) Test_CheckIfEnoughFundsInWallet_TotalAboveWalletButTargetMetricBelow() {
	usecase := suite.makeUsecase()

	wallet := []models.Wallet{
		{
			Id:           "wallet-1",
			Status:       "active",
			BalanceCents: 8000,
		},
	}

	subscriptions := []models.Subscription{
		{
			Id:         "sub-1",
			ExternalId: "ext-sub-1",
			Status:     "active",
		},
	}

	detailedSub1 := models.Subscription{
		Id:         "sub-1",
		ExternalId: "ext-sub-1",
		Status:     "active",
		Plan: models.Plan{
			Id:   "plan-1",
			Name: "Basic Plan",
			Charges: []models.Charge{
				{
					Id:                 "charge-1",
					BillableMetricCode: AI_CASE_REVIEW.String(),
				},
				{
					Id:                 "charge-2",
					BillableMetricCode: UNKNOWN.String(),
				},
			},
		},
	}

	// Total is 5000 + 9000 = 14000 > 8000, but AI_CASE_REVIEW alone is 5000 < 8000
	customerUsage := models.CustomerUsage{
		TotalAmountCents: 14000,
		ChargesUsage: []models.ChargeUsage{
			{
				AmountCents: 5000,
				BillableMetric: models.BillableMetricInUsage{
					Code: AI_CASE_REVIEW.String(),
				},
			},
			{
				AmountCents: 9000,
				BillableMetric: models.BillableMetricInUsage{
					Code: UNKNOWN.String(),
				},
			},
		},
	}

	suite.lagoRepository.On("GetWallets", suite.ctx, suite.orgId).Return(wallet, nil)
	suite.lagoRepository.On("GetSubscriptions", suite.ctx, suite.orgId).Return(subscriptions, nil)
	suite.lagoRepository.On("GetSubscription", suite.ctx, "ext-sub-1").Return(detailedSub1, nil)
	suite.lagoRepository.On("GetCustomerUsage", suite.ctx, suite.orgId, "ext-sub-1").Return(customerUsage, nil)

	hasEnoughFunds, subscriptionId, err :=
		usecase.CheckIfEnoughFundsInWallet(suite.ctx, suite.orgId, AI_CASE_REVIEW)

	suite.NoError(err)
	suite.True(hasEnoughFunds, "should return true when total exceeds wallet but AI_CASE_REVIEW usage alone is below wallet balance")
	suite.Equal("ext-sub-1", subscriptionId, "should return the subscription external ID")
	suite.AssertExpectations()
}

// Test CheckIfEnoughFundsInWallet - Total of two charges exceeds wallet, and AI_CASE_REVIEW alone also exceeds wallet
func (suite *LagoBillingUsecaseTestSuite) Test_CheckIfEnoughFundsInWallet_TotalAboveWalletAndTargetMetricAbove() {
	usecase := suite.makeUsecase()

	wallet := []models.Wallet{
		{
			Id:           "wallet-1",
			Status:       "active",
			BalanceCents: 8000,
		},
	}

	subscriptions := []models.Subscription{
		{
			Id:         "sub-1",
			ExternalId: "ext-sub-1",
			Status:     "active",
		},
	}

	detailedSub1 := models.Subscription{
		Id:         "sub-1",
		ExternalId: "ext-sub-1",
		Status:     "active",
		Plan: models.Plan{
			Id:   "plan-1",
			Name: "Basic Plan",
			Charges: []models.Charge{
				{
					Id:                 "charge-1",
					BillableMetricCode: AI_CASE_REVIEW.String(),
				},
				{
					Id:                 "charge-2",
					BillableMetricCode: UNKNOWN.String(),
				},
			},
		},
	}

	// Total is 9000 + 3000 = 12000 > 8000, and AI_CASE_REVIEW alone is 9000 > 8000
	customerUsage := models.CustomerUsage{
		TotalAmountCents: 12000,
		ChargesUsage: []models.ChargeUsage{
			{
				AmountCents: 9000,
				BillableMetric: models.BillableMetricInUsage{
					Code: AI_CASE_REVIEW.String(),
				},
			},
			{
				AmountCents: 3000,
				BillableMetric: models.BillableMetricInUsage{
					Code: UNKNOWN.String(),
				},
			},
		},
	}

	suite.lagoRepository.On("GetWallets", suite.ctx, suite.orgId).Return(wallet, nil)
	suite.lagoRepository.On("GetSubscriptions", suite.ctx, suite.orgId).Return(subscriptions, nil)
	suite.lagoRepository.On("GetSubscription", suite.ctx, "ext-sub-1").Return(detailedSub1, nil)
	suite.lagoRepository.On("GetCustomerUsage", suite.ctx, suite.orgId, "ext-sub-1").Return(customerUsage, nil)

	hasEnoughFunds, subscriptionId, err :=
		usecase.CheckIfEnoughFundsInWallet(suite.ctx, suite.orgId, AI_CASE_REVIEW)

	suite.NoError(err)
	suite.False(hasEnoughFunds, "should return false when both total and AI_CASE_REVIEW usage exceed wallet balance")
	suite.Empty(subscriptionId, "should return empty subscription ID when not enough funds")
	suite.AssertExpectations()
}
