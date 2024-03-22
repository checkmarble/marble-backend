package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBTransferMapping struct {
	Id               string    `db:"id"`
	CreatedAt        time.Time `db:"created_at"`
	OrganizationId   string    `db:"organization_id"`
	ClientTransferId string    `db:"client_transfer_id"`
}

const TABLE_TRANSFER_MAPPINGS = "transfer_mappings"

var SelectTransferMappingsColumn = utils.ColumnList[DBTransferMapping]()

func AdaptTransferMapping(db DBTransferMapping) (models.TransferMapping, error) {
	return models.TransferMapping(db), nil
}
