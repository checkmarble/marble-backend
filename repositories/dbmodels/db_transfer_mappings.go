package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBTransferMapping struct {
	Id               string    `db:"id"`
	ClientTransferId string    `db:"client_transfer_id"`
	CreatedAt        time.Time `db:"created_at"`
	OrganizationId   string    `db:"organization_id"`
	PartnerId        string    `db:"partner_id"`
}

const TABLE_TRANSFER_MAPPINGS = "transfer_mappings"

var SelectTransferMappingsColumn = utils.ColumnList[DBTransferMapping]()

func AdaptTransferMapping(db DBTransferMapping) (models.TransferMapping, error) {
	return models.TransferMapping(db), nil
}
