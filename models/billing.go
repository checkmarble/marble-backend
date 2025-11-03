package models

import (
	"time"

	"github.com/cockroachdb/errors"
)

type WalletStatus string

const (
	WalletStatusActive     WalletStatus = "active"
	WalletStatusTerminated WalletStatus = "terminated"
)

func WalletStatusFromString(s string) (WalletStatus, error) {
	switch s {
	case "active":
		return WalletStatusActive, nil
	case "terminated":
		return WalletStatusTerminated, nil
	default:
		return "", errors.Newf("invalid wallet status: %s", s)
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
