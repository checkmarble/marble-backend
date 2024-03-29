package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/guregu/null/v5"
)

var TransferStatuses = []string{"neutral", "suspected_fraud", "confirmed_fraud"}

type Transfer struct {
	Id           string
	LastScoredAt null.Time
	Score        null.Int32
	TransferData TransferData
}

type TransferMapping struct {
	Id               string
	ClientTransferId string
	CreatedAt        time.Time
	OrganizationId   string
	PartnerId        string
}

type TransferMappingCreateInput struct {
	ClientTransferId string
	OrganizationId   string
	PartnerId        string
}

type TransferData struct {
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

func TransferFromMap(m map[string]any) (TransferData, error) {
	transfer := TransferData{}
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
	// warning here: reverse of what we do in "ToMap", the struct has "TransferId" but the map has "object_id"
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

type TransferDataCreateBody struct {
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

type TransferCreateOptions struct {
	ComputeScore bool `json:"compute_score"`
}

type TransferCreateBody struct {
	TransferData TransferDataCreateBody
	SkipScore    *bool
}

func (t TransferDataCreateBody) ToIngestionMap(mapping TransferMapping) map[string]any {
	return map[string]any{
		// there is a trap here: we map it to "object_id" to match what we do elsewhere on data model tables
		"object_id": ObjectIdWithPartnerIdPrefix(mapping.PartnerId, t.TransferId),
		// is added to the map to match the data model
		"updated_at": time.Now(),
		// TODO: actually we want this if it's a new transfer, the old value otherwise
		"created_at": time.Now(),
		// marble generated id of the transfer
		"marble_id": mapping.Id,

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

func ObjectIdWithPartnerIdPrefix(partnerId string, transferId string) string {
	return fmt.Sprintf("%s:::%s", partnerId, transferId)
}

type TransferUpdateBody struct {
	Status string
}
