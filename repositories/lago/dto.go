package lago_repository

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type WalletDto struct {
	LagoId       string `json:"lago_id"`
	Status       string `json:"status"`
	Name         string `json:"name"`
	BalanceCents int    `json:"balance_cents"`
}

func AdaptWalletDtoToModel(dto WalletDto) models.Wallet {
	return models.Wallet{
		Id:           dto.LagoId,
		Status:       dto.Status,
		Name:         dto.Name,
		BalanceCents: dto.BalanceCents,
	}
}

type WalletsDto struct {
	Wallets []WalletDto `json:"wallets"`
}

func AdaptWalletsDtoToModel(dto WalletsDto) []models.Wallet {
	return pure_utils.Map(dto.Wallets, AdaptWalletDtoToModel)
}

type ChargeDto struct {
	LagoId             string `json:"lago_id"`
	BillableMetricCode string `json:"billable_metric_code"`
}

func AdaptChargeDtoToModel(dto ChargeDto) models.Charge {
	return models.Charge{
		Id:                 dto.LagoId,
		BillableMetricCode: dto.BillableMetricCode,
	}
}

type PlanDto struct {
	LagoId  string      `json:"lago_id"`
	Name    string      `json:"name"`
	Charges []ChargeDto `json:"charges"`
}

func AdaptPlanDtoToModel(dto PlanDto) models.Plan {
	return models.Plan{
		Id:      dto.LagoId,
		Name:    dto.Name,
		Charges: pure_utils.Map(dto.Charges, AdaptChargeDtoToModel),
	}
}

// Detailed subscription
type SubscriptionDto struct {
	Subscription struct {
		LagoId     string  `json:"lago_id"`
		ExternalId string  `json:"external_id"`
		Status     string  `json:"status"`
		Plan       PlanDto `json:"plan"`
	} `json:"subscription"`
}

func AdaptSubscriptionDtoToModel(dto SubscriptionDto) models.Subscription {
	return models.Subscription{
		Id:         dto.Subscription.LagoId,
		ExternalId: dto.Subscription.ExternalId,
		Status:     dto.Subscription.Status,
		Plan:       AdaptPlanDtoToModel(dto.Subscription.Plan),
	}
}

type SubscriptionsItemDto struct {
	LagoId     string `json:"lago_id"`
	ExternalId string `json:"external_id"`
	Status     string `json:"status"`
}

func AdaptSubscriptionsItemDtoToModel(dto SubscriptionsItemDto) models.Subscription {
	return models.Subscription{
		Id:         dto.LagoId,
		ExternalId: dto.ExternalId,
		Status:     dto.Status,
	}
}

// Summary of subscriptions for an organization
type SubscriptionsDto struct {
	Subscriptions []SubscriptionsItemDto `json:"subscriptions"`
}

func AdaptSubscriptionsDtoToModel(dto SubscriptionsDto) []models.Subscription {
	return pure_utils.Map(dto.Subscriptions, AdaptSubscriptionsItemDtoToModel)
}

type CustomerUsageDto struct {
	CustomerUsage struct {
		TotalAmountCents int `json:"total_amount_cents"`
	} `json:"customer_usage"`
}

func AdaptCustomerUsageDtoToModel(dto CustomerUsageDto) models.CustomerUsage {
	return models.CustomerUsage{
		TotalAmountCents: dto.CustomerUsage.TotalAmountCents,
	}
}

type BillingEventItemDto struct {
	TransactionId          string         `json:"transaction_id"`
	ExternalSubscriptionId string         `json:"external_subscription_id"`
	Code                   string         `json:"code"`
	Timestamp              int64          `json:"timestamp"`
	Properties             map[string]any `json:"properties"`
}

func AdaptModelToBillingEventItemDto(event models.BillingEvent) BillingEventItemDto {
	return BillingEventItemDto{
		TransactionId:          event.TransactionId,
		ExternalSubscriptionId: event.ExternalSubscriptionId,
		Code:                   event.Code,
		Timestamp:              event.Timestamp.Unix(),
		Properties:             event.Properties,
	}
}

type BillingEventDto struct {
	Event BillingEventItemDto `json:"event"`
}

func AdaptModelToBillingEventDto(event models.BillingEvent) BillingEventDto {
	return BillingEventDto{
		Event: AdaptModelToBillingEventItemDto(event),
	}
}

type BillingEventsDto struct {
	Events []BillingEventItemDto `json:"events"`
}

func AdaptModelToBillingEventsDto(events []models.BillingEvent) BillingEventsDto {
	return BillingEventsDto{
		Events: pure_utils.Map(events, AdaptModelToBillingEventItemDto),
	}
}
