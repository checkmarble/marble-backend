package models

import (
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/guregu/null/v5"
	"github.com/pkg/errors"
)

const (
	TransferAlertStatusPending      = "pending"
	TransferAlertStatusAcknowledged = "acknowledged"
	TransferAlertStatusArchived     = "archived"
)

var TransferAlertStatuses = []string{
	TransferAlertStatusPending,
	TransferAlertStatusAcknowledged,
	TransferAlertStatusArchived,
}

type TransferAlert struct {
	Id                   string
	TransferId           string
	OrganizationId       string
	SenderPartnerId      string
	BeneficiaryPartnerId string
	CreatedAt            time.Time
	Status               string

	// optional
	Message            string
	TransferEndToEndId string
	BeneficiaryIban    string
	SenderIban         string
}

type TransferAlertCreateBody struct {
	TransferId      string
	OrganizationId  string
	SenderPartnerId string

	// optional
	Message            string
	TransferEndToEndId string
	BeneficiaryIban    string
	SenderIban         string
}

func (b TransferAlertCreateBody) WithBeneficiaryPartnerAndDefaults(beneficiaryPartnerId string) (TransferAlert, error) {
	if beneficiaryPartnerId == "" {
		return TransferAlert{}, errors.Wrapf(BadParameterError, "beneficiary_partner_id is required")
	}
	out := TransferAlert{
		Id:                   uuid.New().String(),
		TransferId:           b.TransferId,
		OrganizationId:       b.OrganizationId,
		SenderPartnerId:      b.SenderPartnerId,
		BeneficiaryPartnerId: beneficiaryPartnerId,
		Status:               TransferAlertStatusPending,
		CreatedAt:            time.Now(),

		Message:            b.Message,
		TransferEndToEndId: b.TransferEndToEndId,
		BeneficiaryIban:    b.BeneficiaryIban,
		SenderIban:         b.SenderIban,
	}

	return out, nil
}

func (b TransferAlertCreateBody) Validate() error {
	if b.TransferId == "" {
		return errors.Wrapf(BadParameterError, "transfer_id is required")
	}
	if b.OrganizationId == "" {
		return errors.Wrapf(BadParameterError, "organization_id is required")
	}
	if b.SenderPartnerId == "" {
		return errors.Wrapf(BadParameterError, "sender_partner_id is required")
	}
	if len(b.Message) > 1000 {
		return errors.Wrapf(BadParameterError,
			"message is too long: max length is 1000")
	}
	if len(b.TransferEndToEndId) > 100 {
		return errors.Wrapf(BadParameterError,
			"transfer_end_to_end_id is too long: max length is 100")
	}
	if len(b.BeneficiaryIban) > 34 {
		return errors.Wrapf(BadParameterError,
			"beneficiary_iban is too long: max length is 34")
	}
	if len(b.SenderIban) > 34 {
		return errors.Wrapf(BadParameterError,
			"sender_iban is too long: max length is 34")
	}
	return nil
}

type TransferAlertUpdateBodySender struct {
	Message            null.String
	TransferEndToEndId null.String
	BeneficiaryIban    null.String
	SenderIban         null.String
}

func (b TransferAlertUpdateBodySender) Validate() error {
	if b.Message.Valid && len(b.Message.String) > 1000 {
		return errors.Wrapf(BadParameterError,
			"message is too long: max length is 1000")
	}
	if b.TransferEndToEndId.Valid && len(b.TransferEndToEndId.String) > 100 {
		return errors.Wrapf(BadParameterError,
			"transfer_end_to_end_id is too long: max length is 100")
	}
	if b.BeneficiaryIban.Valid && len(b.BeneficiaryIban.String) > 34 {
		return errors.Wrapf(BadParameterError,
			"beneficiary_iban is too long: max length is 34")
	}
	if b.SenderIban.Valid && len(b.SenderIban.String) > 34 {
		return errors.Wrapf(BadParameterError,
			"sender_iban is too long: max length is 34")
	}
	return nil
}

type TransferAlertUpdateBodyBeneficiary struct {
	Status null.String
}

func (b TransferAlertUpdateBodyBeneficiary) Validate() error {
	if b.Status.Valid && !slices.Contains(TransferAlertStatuses, b.Status.String) {
		return errors.Wrapf(
			BadParameterError,
			"status is invalid: %s", b.Status.String)
	}
	return nil
}
