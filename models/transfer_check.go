package models

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/cockroachdb/errors"
	"github.com/guregu/null/v5"
)

var (
	TransferStatuses = []string{"neutral", "suspected_fraud", "confirmed_fraud"}
	isAlphanumeric   = regexp.MustCompile(`^[a-zA-Z0-9]*$`)
)

const (
	maxStringLengthTfCheck   = 140
	senderAccountIdMaxLength = 50
)

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

func (t TransferDataCreateBody) FormatAndValidate() (TransferDataCreateBody, error) {
	var err error
	var errs []error

	// hash the iban if it's clear - otherwise keep it unchanged
	t.BeneficiaryIban, err = hashIbanIfClear(t.BeneficiaryIban)
	if err != nil {
		errs = append(errs, err)
	}

	// first validate the fields that expect a specific format
	t.BeneficiaryBic, err = formatAndValidateBic(t.BeneficiaryBic)
	errs = append(errs, err)

	t.SenderBic, err = formatAndValidateBic(t.SenderBic)
	errs = append(errs, err)

	t.Currency = strings.ToUpper(t.Currency)
	if !slices.Contains(pure_utils.CurrencyCodes, t.Currency) {
		errs = append(errs, errors.Wrap(BadParameterError, "currency is not valid"))
	}

	_, err = netip.ParseAddr(t.SenderIP)
	if t.SenderIP != "" && err != nil {
		errs = append(errs, errors.Wrap(BadParameterError, "sender_ip is not a valid IP address"))
	}

	if !slices.Contains(TransferStatuses, t.Status) {
		errs = append(errs, errors.Wrap(
			BadParameterError,
			fmt.Sprintf("status %s is not valid", t.Status),
		))
	}

	// max length checks for strings that don't have any other format
	if len(t.BeneficiaryName) > maxStringLengthTfCheck {
		errs = append(errs, errors.Wrap(BadParameterError, "beneficiary_name is too long"))
	}
	if len(t.Label) > maxStringLengthTfCheck {
		errs = append(errs, errors.Wrap(BadParameterError, "label is too long"))
	}
	if len(t.SenderAccountId) > senderAccountIdMaxLength {
		errs = append(errs, errors.Wrap(BadParameterError, "sender_account_id is too long"))
	}
	if len(t.SenderDevice) > maxStringLengthTfCheck {
		errs = append(errs, errors.Wrap(BadParameterError, "sender_device is too long"))
	}
	if t.Value <= 0 {
		errs = append(errs, errors.Wrap(BadParameterError, "value must be positive"))
	}

	return t, errors.Join(errs...)
}

func formatAndValidateBic(bic string) (string, error) {
	bic = strings.ToUpper(strings.TrimSpace(bic))
	if len(bic) != 8 && len(bic) != 11 {
		return "", errors.Wrap(BadParameterError, "bic must be 8 or 11 characters")
	}
	bicIsAlphanumeric := isAlphanumeric.MatchString(bic)
	if !bicIsAlphanumeric {
		return "", errors.Wrap(BadParameterError, "bic must be alphanumeric")
	}

	return bic[:8], nil
}

// takes a clear iban or a hexadecimal hash of an iban and returns a hexadecimal hash of the iban
func hashIbanIfClear(ibanOrHash string) (string, error) {
	data, err := hex.DecodeString(ibanOrHash)
	if err == nil {
		if len(data) == sha256.Size {
			return trimAndUpper(ibanOrHash), nil
		}
	}

	iban := trimAndUpper(ibanOrHash)
	if !isAlphanumeric.MatchString(iban) {
		return "", errors.Wrap(BadParameterError, "iban must be alphanumeric")
	}
	if len(iban) < 15 || len(iban) > 34 {
		return "", errors.Wrap(BadParameterError, "iban must be between 15 and 34 characters")
	}

	hash := sha256.Sum256([]byte(trimAndUpper(iban)))
	return strings.ToUpper(hex.EncodeToString(hash[:])), nil
}

func trimAndUpper(s string) string {
	return strings.TrimSpace(strings.ToUpper(s))
}

func ObjectIdWithPartnerIdPrefix(partnerId string, transferId string) string {
	return fmt.Sprintf("%s:::%s", partnerId, transferId)
}

type TransferUpdateBody struct {
	Status string
}
