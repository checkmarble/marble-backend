package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_SANCTION_CHECK_WHITELISTS = "sanction_check_whitelists"

type DBSanctionCheckWhitelists struct {
	Id             string    `db:"id"`
	OrgId          string    `db:"org_id"`
	CounterpartyId string    `db:"counterparty_id"`
	EntityId       string    `db:"entity_id"`
	WhitelistedBy  *string   `db:"whitelisted_by"`
	CreatedAt      time.Time `db:"created_at"`
}

var SanctionCheckWhitelistColumnList = utils.ColumnList[DBSanctionCheckWhitelists]()

func AdaptSanctionCheckWhitelist(db DBSanctionCheckWhitelists) (models.SanctionCheckWhitelist, error) {
	return models.SanctionCheckWhitelist{
		Id:             db.Id,
		OrgId:          db.OrgId,
		CounterpartyId: db.CounterpartyId,
		EntityId:       db.EntityId,
		WhitelistedBy:  db.WhitelistedBy,
		CreatedAt:      db.CreatedAt,
	}, nil
}
