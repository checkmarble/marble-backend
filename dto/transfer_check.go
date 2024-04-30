package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/guregu/null/v5"
)

type Transfer struct {
	Id           string       `json:"id"`
	LastScoredAt null.Time    `json:"last_scored_at"`
	Score        null.Int32   `json:"score"`
	TransferData TransferData `json:"transfer_data"`
}

func AdaptTransferCheckResultDto(result models.Transfer) Transfer {
	return Transfer{
		Id:           result.Id,
		LastScoredAt: result.LastScoredAt,
		Score:        result.Score,
		TransferData: AdaptTransferDataDto(result.TransferData),
	}
}

type TransferData struct {
	BeneficiaryBic      string    `json:"beneficiary_bic"`
	BeneficiaryIban     string    `json:"beneficiary_iban"`
	BeneficiaryName     string    `json:"beneficiary_name"`
	CreatedAt           time.Time `json:"created_at"`
	Currency            string    `json:"currency"`
	Label               string    `json:"label"`
	SenderAccountId     string    `json:"sender_account_id"`
	SenderBic           string    `json:"sender_bic"`
	SenderDevice        string    `json:"sender_device"`
	SenderIP            string    `json:"sender_ip"`
	Status              string    `json:"status"`
	Timezone            string    `json:"timezone"`
	TransferId          string    `json:"transfer_id"`
	TransferRequestedAt time.Time `json:"transfer_requested_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	Value               int64     `json:"value"`
}

func AdaptTransferDataDto(transfer models.TransferData) TransferData {
	return TransferData{
		BeneficiaryBic:      transfer.BeneficiaryBic,
		BeneficiaryIban:     transfer.BeneficiaryIban,
		BeneficiaryName:     transfer.BeneficiaryName,
		CreatedAt:           transfer.CreatedAt,
		Currency:            transfer.Currency,
		Label:               transfer.Label,
		SenderAccountId:     transfer.SenderAccountId,
		SenderBic:           transfer.SenderBic,
		SenderDevice:        transfer.SenderDevice,
		SenderIP:            transfer.SenderIP,
		Status:              transfer.Status,
		Timezone:            transfer.Timezone,
		TransferId:          transfer.TransferId,
		TransferRequestedAt: transfer.TransferRequestedAt,
		UpdatedAt:           transfer.UpdatedAt,
		Value:               transfer.Value,
	}
}

type TransferDataCreateBody struct {
	BeneficiaryBic      string    `json:"beneficiary_bic" binding:"required"`
	BeneficiaryIban     string    `json:"beneficiary_iban" binding:"required"`
	BeneficiaryName     string    `json:"beneficiary_name"`
	Currency            string    `json:"currency" binding:"required"`
	Label               string    `json:"label"`
	SenderAccountId     string    `json:"sender_account_id" binding:"required"`
	SenderBic           string    `json:"sender_bic" binding:"required"`
	SenderDevice        string    `json:"sender_device"`
	SenderIP            string    `json:"sender_ip"`
	Status              string    `json:"status"`
	Timezone            string    `json:"timezone"`
	TransferId          string    `json:"transfer_id" binding:"required"`
	TransferRequestedAt time.Time `json:"transfer_requested_at" binding:"required"`
	Value               int64     `json:"value" binding:"required"`
}

type TransferCreateBody struct {
	TransferData *TransferDataCreateBody `json:"transfer_data" binding:"required"`
	SkipScore    *bool                   `json:"skip_score"`
}

func AdaptTransferDataCreateBody(body TransferDataCreateBody) models.TransferDataCreateBody {
	return models.TransferDataCreateBody(body)
}

func AdaptTransferCreateBody(body TransferCreateBody) models.TransferCreateBody {
	return models.TransferCreateBody{
		TransferData: AdaptTransferDataCreateBody(*body.TransferData),
		SkipScore:    body.SkipScore,
	}
}

type TransferUpdateBody struct {
	Status string `json:"status" binding:"required"`
}
