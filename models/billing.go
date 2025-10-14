package models

import "time"

type Wallet struct {
	Id              string
	Status          string
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

type CustomerUsage struct {
	TotalAmountCents int
}

type BillingEvent struct {
	TransactionId          string
	ExternalSubscriptionId string
	Code                   string
	Timestamp              *time.Time
	Properties             map[string]any
}
