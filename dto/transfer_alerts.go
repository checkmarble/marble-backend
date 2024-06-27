package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/guregu/null/v5"
)

type TransferAlertSender struct {
	Id                 string    `json:"id"`
	TransferId         string    `json:"transfer_id"`
	SenderPartnerId    string    `json:"sender_partner_id"`
	CreatedAt          time.Time `json:"created_at"`
	Status             string    `json:"status"`
	Message            string    `json:"message"`
	TransferEndToEndId string    `json:"transfer_end_to_end_id"`
	BeneficiaryIban    string    `json:"beneficiary_iban"`
	SenderIban         string    `json:"sender_iban"`
}

type TransferAlertBeneficiary struct {
	Id                   string    `json:"id"`
	BeneficiaryPartnerId string    `json:"beneficiary_partner_id"`
	CreatedAt            time.Time `json:"created_at"`
	Status               string    `json:"status"`
	Message              string    `json:"message"`
	TransferEndToEndId   string    `json:"transfer_end_to_end_id"`
	BeneficiaryIban      string    `json:"beneficiary_iban"`
	SenderIban           string    `json:"sender_iban"`
}

func AdaptSenderTransferAlert(alert models.TransferAlert) TransferAlertSender {
	return TransferAlertSender{
		Id:                 alert.Id,
		TransferId:         alert.TransferId,
		SenderPartnerId:    alert.SenderPartnerId,
		CreatedAt:          alert.CreatedAt,
		Status:             alert.Status,
		Message:            alert.Message,
		TransferEndToEndId: alert.TransferEndToEndId,
		BeneficiaryIban:    alert.BeneficiaryIban,
		SenderIban:         alert.SenderIban,
	}
}

func AdaptBeneficiaryTransferAlert(alert models.TransferAlert) TransferAlertBeneficiary {
	return TransferAlertBeneficiary{
		Id:                   alert.Id,
		BeneficiaryPartnerId: alert.BeneficiaryPartnerId,
		CreatedAt:            alert.CreatedAt,
		Status:               alert.Status,
		Message:              alert.Message,
		TransferEndToEndId:   alert.TransferEndToEndId,
		BeneficiaryIban:      alert.BeneficiaryIban,
		SenderIban:           alert.SenderIban,
	}
}

type TransferAlertCreateBody struct {
	TransferId         string `json:"transfer_id" binding:"required"`
	Message            string `json:"message"`
	TransferEndToEndId string `json:"transfer_end_to_end_id"`
	BeneficiaryIban    string `json:"beneficiary_iban"`
	SenderIban         string `json:"sender_iban"`
}

type TransferAlertUpdateBody struct {
	Status             null.String `json:"status"`
	Message            null.String `json:"message"`
	TransferEndToEndId null.String `json:"transfer_end_to_end_id"`
	BeneficiaryIban    null.String `json:"beneficiary_iban"`
	SenderIban         null.String `json:"sender_iban"`
}
