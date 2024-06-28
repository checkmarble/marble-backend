package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBTransferAlert struct {
	Id                   string    `db:"id"`
	TransferId           string    `db:"transfer_id"`
	OrganizationId       string    `db:"organization_id"`
	SenderPartnerId      string    `db:"sender_partner_id"`
	BeneficiaryPartnerId string    `db:"beneficiary_partner_id"`
	CreatedAt            time.Time `db:"created_at"`
	Status               string    `db:"status"`

	Message            string `db:"message"`
	TransferEndToEndId string `db:"transfer_end_to_end_id"`
	BeneficiaryIban    string `db:"beneficiary_iban"`
	SenderIban         string `db:"sender_iban"`
}

const TABLE_TRANSFER_ALERTS = "transfer_alerts"

var SelectTransferAlertsColumn = utils.ColumnList[DBTransferAlert]()

func AdaptTransferAlert(db DBTransferAlert) (models.TransferAlert, error) {
	return models.TransferAlert(db), nil
}
