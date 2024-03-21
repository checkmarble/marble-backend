package models

import (
	"errors"
	"time"

	"github.com/guregu/null/v5"
)

type TransferCheckScoreDetail struct {
	Score        null.Int32
	LastScoredAt null.Time
}

type TransferCheckResult struct {
	Result   TransferCheckScoreDetail
	Transfer Transfer
}

type Transfer struct {
	BeneficiaryBic      string
	BeneficiaryIban     string
	BeneficiaryName     string
	CreatedAt           time.Time
	Currency            string
	Label               string
	SenderAccountId     string
	SenderBic           string
	SenderDevice        string
	SenderIP            string
	Status              string
	Timezone            string
	TransferId          string
	TransferRequestedAt time.Time
	UpdatedAt           time.Time
	Value               int64
}

func TransferFromMap(m map[string]any) (Transfer, error) {
	transfer := Transfer{}
	var ok bool
	transfer.BeneficiaryBic, ok = m["beneficiary_bic"].(string)
	if !ok {
		return transfer, errors.New("beneficiary_bic is not a string")
	}
	transfer.BeneficiaryIban, ok = m["beneficiary_iban"].(string)
	if !ok {
		return transfer, errors.New("beneficiary_iban is not a string")
	}
	transfer.BeneficiaryName, ok = m["beneficiary_name"].(string)
	if !ok {
		return transfer, errors.New("beneficiary_name is not a string")
	}
	transfer.CreatedAt, ok = m["created_at"].(time.Time)
	if !ok {
		return transfer, errors.New("created_at is not a time.Time")
	}
	transfer.Currency, ok = m["currency"].(string)
	if !ok {
		return transfer, errors.New("currency is not a string")
	}
	transfer.Label, ok = m["label"].(string)
	if !ok {
		return transfer, errors.New("label is not a string")
	}
	transfer.SenderAccountId, ok = m["sender_account_id"].(string)
	if !ok {
		return transfer, errors.New("sender_account_id is not a string")
	}
	transfer.SenderBic, ok = m["sender_bic"].(string)
	if !ok {
		return transfer, errors.New("sender_bic is not a string")
	}
	transfer.SenderDevice, ok = m["sender_device"].(string)
	if !ok {
		return transfer, errors.New("sender_device is not a string")
	}
	transfer.SenderIP, ok = m["sender_ip"].(string)
	if !ok {
		return transfer, errors.New("sender_ip is not a string")
	}
	transfer.Status, ok = m["status"].(string)
	if !ok {
		return transfer, errors.New("status is not a string")
	}
	transfer.Timezone, ok = m["timezone"].(string)
	if !ok {
		return transfer, errors.New("timezone is not a string")
	}
	transfer.TransferId, ok = m["object_id"].(string)
	if !ok {
		return transfer, errors.New("object_id is not a string")
	}
	transfer.TransferRequestedAt, ok = m["transfer_requested_at"].(time.Time)
	if !ok {
		return transfer, errors.New("transfer_requested_at is not a time.Time")
	}
	transfer.UpdatedAt, ok = m["updated_at"].(time.Time)
	if !ok {
		return transfer, errors.New("updated_at is not a time.Time")
	}
	transfer.Value, ok = m["value"].(int64)
	if !ok {
		return transfer, errors.New("value is not an int64")
	}
	return transfer, nil
}

type TransferCheckCreateBody struct {
	BeneficiaryBic      string
	BeneficiaryIban     string
	BeneficiaryName     string
	Currency            string
	Label               string
	SenderAccountId     string
	SenderBic           string
	SenderDevice        string
	SenderIP            string
	Status              string
	Timezone            string
	TransferId          string
	TransferRequestedAt time.Time
	Value               int64
}

func (t TransferCheckCreateBody) ToMap() map[string]any {
	return map[string]any{
		// there is a trap here: we map it to "object_id" to match what we do elsewhere on data model tables
		"object_id": t.TransferId,
		// is added to the map to match the data model
		"updated_at": time.Now(),
		// TODO: actually we want this if it's a new transfer, the old value otherwise
		"created_at": time.Now(),

		"beneficiary_bic":       t.BeneficiaryBic,
		"beneficiary_iban":      t.BeneficiaryIban,
		"beneficiary_name":      t.BeneficiaryName,
		"currency":              t.Currency,
		"label":                 t.Label,
		"sender_account_id":     t.SenderAccountId,
		"sender_bic":            t.SenderBic,
		"sender_device":         t.SenderDevice,
		"sender_ip":             t.SenderIP,
		"status":                t.Status,
		"timezone":              t.Timezone,
		"transfer_requested_at": t.TransferRequestedAt,
		"value":                 t.Value,
	}
}
