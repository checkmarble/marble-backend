package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/guregu/null/v5"
)

type TransferCheckScoreDetail struct {
	Score        null.Int32 `json:"score"`
	LastScoredAt null.Time  `json:"last_scored_at"`
}

type TransferCheckResult struct {
	Result   TransferCheckScoreDetail `json:"result"`
	Transfer Transfer                 `json:"transfer"`
}

func AdaptTransferCheckResultDto(result models.TransferCheckResult) TransferCheckResult {
	return TransferCheckResult{
		Result:   AdaptTransferCheckScoreDetailDto(result.Result),
		Transfer: AdaptTransferDto(result.Transfer),
	}
}

func AdaptTransferCheckScoreDetailDto(scoreDetail models.TransferCheckScoreDetail) TransferCheckScoreDetail {
	return TransferCheckScoreDetail{
		Score:        scoreDetail.Score,
		LastScoredAt: scoreDetail.LastScoredAt,
	}
}

type Transfer struct {
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

func AdaptTransferDto(transfer models.Transfer) Transfer {
	return Transfer{
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

func AdaptTransfer(transfer Transfer) models.Transfer {
	return models.Transfer{
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

type TransferCheckCreateBody struct {
	BeneficiaryBic      string    `json:"beneficiary_bic"`
	BeneficiaryIban     string    `json:"beneficiary_iban"`
	BeneficiaryName     string    `json:"beneficiary_name"`
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
	Value               int64     `json:"value"`
}

func AdaptTransferCheckCreateBody(body TransferCheckCreateBody) models.TransferCheckCreateBody {
	return models.TransferCheckCreateBody{
		BeneficiaryBic:      body.BeneficiaryBic,
		BeneficiaryIban:     body.BeneficiaryIban,
		BeneficiaryName:     body.BeneficiaryName,
		Currency:            body.Currency,
		Label:               body.Label,
		SenderAccountId:     body.SenderAccountId,
		SenderBic:           body.SenderBic,
		SenderDevice:        body.SenderDevice,
		SenderIP:            body.SenderIP,
		Status:              body.Status,
		Timezone:            body.Timezone,
		TransferId:          body.TransferId,
		TransferRequestedAt: body.TransferRequestedAt,
		Value:               body.Value,
	}
}
