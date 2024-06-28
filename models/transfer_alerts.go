package models

import (
	"time"

	"github.com/guregu/null/v5"
)

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
	Id                   string
	TransferId           string
	OrganizationId       string
	SenderPartnerId      string
	BeneficiaryPartnerId string

	// optional
	Message            string
	TransferEndToEndId string
	BeneficiaryIban    string
	SenderIban         string
}

type TransferAlertUpdateBodySender struct {
	Message            null.String
	TransferEndToEndId null.String
	BeneficiaryIban    null.String
	SenderIban         null.String
}

type TransferAlertUpdateBodyReceiver struct {
	Status null.String
}
