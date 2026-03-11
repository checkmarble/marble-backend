package models

import (
	"time"
)

type WalletStatus string

const (
	WalletStatusUnknown    WalletStatus = "unknown"
	WalletStatusActive     WalletStatus = "active"
	WalletStatusTerminated WalletStatus = "terminated"
)

func WalletStatusFromString(s string) WalletStatus {
	switch s {
	case "active":
		return WalletStatusActive
	case "terminated":
		return WalletStatusTerminated
	default:
		return WalletStatusUnknown
	}
}

func (s WalletStatus) String() string {
	return string(s)
}

type Wallet struct {
	Id              string
	Status          WalletStatus
	Name            string
	CreditsBalance  float64
	BalanceCents    int
	ConsumedCredits float64
}

type Charge struct {
	Id                 string
	BillableMetricCode string
}

type Plan struct {
	Id      string
	Name    string
	Charges []Charge
}

type Subscription struct {
	Id         string
	ExternalId string
	Status     string
	Plan       Plan
}

type BillableMetricInUsage struct {
	LagoId          string
	Code            string
	Name            string
	AggregationType string
}

type ChargeUsage struct {
	AmountCents    int
	BillableMetric BillableMetricInUsage
}

type CustomerUsage struct {
	TotalAmountCents int
	ChargesUsage     []ChargeUsage
}

type BillingEvent struct {
	TransactionId          string
	ExternalSubscriptionId string
	Code                   string
	Timestamp              *time.Time
	Properties             map[string]any
}

type BillingEntitlement struct {
	Code string
}
