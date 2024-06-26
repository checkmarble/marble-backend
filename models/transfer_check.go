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

const (
	TransferStatusNeutral        = "neutral"
	TransferStatusSuspectedFraud = "suspected_fraud"
	TransferStatusConfirmedFraud = "confirmed_fraud"
)

var (
	TransferStatuses   = []string{TransferStatusNeutral, TransferStatusSuspectedFraud, TransferStatusConfirmedFraud}
	SenderAccountTypes = []string{"physical_person", "moral_person"}
	isAlphanumeric     = regexp.MustCompile(`^[a-zA-Z0-9]*$`)
)

const (
	maxStringLengthTfCheck = 140
	idMaxLength            = 50
)

const (
	RegularIP = "regular"
	TorIP     = "tor"
	VpnIP     = "vpn"

	TrustedSender = "trusted"
	RegularSender = "regular"

	TransferCheckTable = "transfers"
)

type Transfer struct {
	Id                   string
	LastScoredAt         null.Time
	Score                null.Int32
	TransferData         TransferData
	BeneficiaryInNetwork bool
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
	SenderAccountType   string
	SenderBic           string
	SenderBicRiskLevel  string
	SenderDevice        string
	SenderIP            netip.Addr
	SenderIPType        string
	SenderIPCountry     string
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
	transfer.SenderAccountType, ok = m["sender_account_type"].(string)
	if !ok {
		return transfer, errors.New("sender_account_type is not a string")
	}
	transfer.SenderBic, ok = m["sender_bic"].(string)
	if !ok {
		return transfer, errors.New("sender_bic is not a string")
	}
	bicRiskLevel, found := m["sender_bic_risk_level"]
	if found {
		transfer.SenderBicRiskLevel, ok = bicRiskLevel.(string)
		if !ok {
			return transfer, errors.New("sender_bic_risk_level is not a string")
		}
	}
	transfer.SenderDevice, ok = m["sender_device"].(string)
	if !ok {
		return transfer, errors.New("sender_device is not a string")
	}

	ipString, ok := m["sender_ip"].(string)
	if !ok {
		return transfer, errors.New("sender_ip is not a string")
	}
	if ipString != "" {
		ip, err := netip.ParseAddr(ipString)
		if err != nil {
			transfer.SenderIP = netip.IPv4Unspecified()
		} else {
			transfer.SenderIP = ip
		}
	}

	senderIpType, found := m["sender_ip_type"]
	if found {
		transfer.SenderIPType, ok = senderIpType.(string)
		if !ok {
			return transfer, errors.New("sender_ip_type is not a string")
		}
	}
	senderIpCountry, found := m["sender_ip_country"]
	if found {
		transfer.SenderIPCountry, ok = senderIpCountry.(string)
		if !ok {
			return transfer, errors.New("sender_ip_country is not a string")
		}
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
	SenderAccountType   string
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

func (t TransferData) ToIngestionMap(mapping TransferMapping) map[string]any {
	return map[string]any{
		// there is a trap here: we map it to "object_id" to match what we do elsewhere on data model tables
		"object_id": ObjectIdWithPartnerIdPrefix(mapping.PartnerId, t.TransferId),
		// marble generated id of the transfer
		"marble_id": mapping.Id,

		"beneficiary_bic":       t.BeneficiaryBic,
		"beneficiary_iban":      t.BeneficiaryIban,
		"beneficiary_name":      t.BeneficiaryName,
		"created_at":            t.CreatedAt,
		"currency":              t.Currency,
		"label":                 t.Label,
		"sender_account_id":     t.SenderAccountId,
		"sender_account_type":   t.SenderAccountType,
		"sender_bic":            t.SenderBic,
		"sender_bic_risk_level": t.SenderBicRiskLevel,
		"sender_device":         t.SenderDevice,
		"sender_ip":             t.SenderIP.String(),
		"sender_ip_type":        t.SenderIPType,
		"sender_ip_country":     t.SenderIPCountry,
		"status":                t.Status,
		"timezone":              t.Timezone,
		"transfer_requested_at": t.TransferRequestedAt,
		"updated_at":            t.UpdatedAt,
		"value":                 t.Value,
	}
}

func (t TransferDataCreateBody) FormatAndValidate() (TransferData, error) {
	var err error
	errs := make(FieldValidationError, 10)
	out := TransferData{
		BeneficiaryBic:      t.BeneficiaryBic,
		BeneficiaryIban:     t.BeneficiaryIban,
		BeneficiaryName:     t.BeneficiaryName,
		CreatedAt:           time.Now(),
		Currency:            t.Currency,
		Label:               t.Label,
		SenderAccountId:     t.SenderAccountId,
		SenderAccountType:   t.SenderAccountType,
		SenderBic:           t.SenderBic,
		SenderBicRiskLevel:  RegularSender,
		SenderDevice:        t.SenderDevice,
		SenderIP:            netip.IPv4Unspecified(), // SenderIP defaults to 0.0.0.0 if not provided
		SenderIPType:        RegularIP,
		SenderIPCountry:     "FR",
		Status:              t.Status,
		Timezone:            t.Timezone,
		TransferId:          t.TransferId,
		TransferRequestedAt: t.TransferRequestedAt,
		UpdatedAt:           time.Now(),
		Value:               t.Value,
	}

	// hash the iban if it's clear - otherwise keep it unchanged
	out.BeneficiaryIban, err = validateIbanOrHashIfClear(t.BeneficiaryIban)
	if err != nil {
		errs["beneficiary_iban"] = err.Error()
	}

	// first validate the fields that expect a specific format
	out.BeneficiaryBic, err = formatAndValidateBic(t.BeneficiaryBic)
	if err != nil {
		errs["beneficiary_bic"] = err.Error()
	}

	out.SenderBic, err = formatAndValidateBic(t.SenderBic)
	if err != nil {
		errs["sender_bic"] = err.Error()
	}

	out.Currency = strings.ToUpper(t.Currency)
	if !slices.Contains(pure_utils.CurrencyCodes, t.Currency) {
		errs["currency"] = fmt.Sprintf("currency '%s' is not valid", t.Currency)
	}

	if t.SenderIP != "" {
		ip, err := netip.ParseAddr(t.SenderIP)
		if err != nil {
			errs["sender_ip"] = fmt.Sprintf("sender_ip '%s' is not a valid IP address", t.SenderIP)
		} else {
			out.SenderIP = ip
		}
	}

	if t.Status == "" {
		out.Status = TransferStatusNeutral
	} else if !slices.Contains(TransferStatuses, t.Status) {
		errs["status"] = fmt.Sprintf("status '%s' is not valid", t.Status)
	}

	if !slices.Contains(SenderAccountTypes, t.SenderAccountType) {
		errs["sender_account_type"] = fmt.Sprintf("sender_account_type '%s' is not valid", t.SenderAccountType)
	}

	stringTooLongErr := "string is too long"
	// max length checks for strings that don't have any other format
	if len(t.BeneficiaryName) > maxStringLengthTfCheck {
		errs["beneficiary_name"] = stringTooLongErr
	}
	if len(t.Label) > maxStringLengthTfCheck {
		errs["label"] = stringTooLongErr
	}
	if len(t.SenderAccountId) > idMaxLength {
		errs["sender_account_id"] = stringTooLongErr
	}
	if len(t.TransferId) > idMaxLength {
		errs["transfer_id"] = stringTooLongErr
	}
	if len(t.SenderDevice) > maxStringLengthTfCheck {
		errs["sender_device"] = stringTooLongErr
	}
	if t.Value <= 0 {
		errs["value"] = "value must be positive"
	}

	if t.Timezone == "" {
		out.Timezone = "Europe/Paris"
	} else if t.Timezone != "Europe/Paris" {
		errs["timezone"] = "timezone must be Europe/Paris"
	}

	// FieldValidationError implements the error interface, but I created a non-nil map to fill it
	// so we only want to return it if any errors were added
	if len(errs) > 0 {
		return out, errs
	}
	return out, nil
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
// In detail:
// - if it looks like a SHA256 hash, keep it and upper case it
// - otherwise
//   - upper case it and remove spaces
//   - check if it's alphanumeric & if it's between 15 and 34 characters
//   - if it is, hash it with SHA256 and return the hash as a hexadecimal string (upper case)
func validateIbanOrHashIfClear(ibanOrHash string) (string, error) {
	data, err := hex.DecodeString(ibanOrHash)
	if err == nil {
		if len(data) == sha256.Size {
			return strings.ToUpper(ibanOrHash), nil
		}
	}

	iban := strings.ToUpper(strings.ReplaceAll(ibanOrHash, " ", ""))
	if !isAlphanumeric.MatchString(iban) {
		return "", errors.Wrap(BadParameterError, "iban must be alphanumeric")
	}
	if len(iban) < 15 || len(iban) > 34 {
		return "", errors.Wrap(BadParameterError, "iban must be between 16 and 34 characters")
	}

	hash := sha256.Sum256([]byte(iban))
	return strings.ToUpper(hex.EncodeToString(hash[:])), nil
}

func ObjectIdWithPartnerIdPrefix(partnerId string, transferId string) string {
	return fmt.Sprintf("%s:::%s", partnerId, transferId)
}

type TransferUpdateBody struct {
	Status string
}
