package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const TABLE_SCREENING_WHITELISTS = "screening_whitelists"

type DBScreeningWhitelists struct {
	Id             string    `db:"id"`
	OrgId          uuid.UUID `db:"org_id"`
	CounterpartyId string    `db:"counterparty_id"`
	EntityId       string    `db:"entity_id"`
	WhitelistedBy  *string   `db:"whitelisted_by"`
	CreatedAt      time.Time `db:"created_at"`
}

var ScreeningWhitelistColumnList = utils.ColumnList[DBScreeningWhitelists]()

func AdaptScreeningWhitelist(db DBScreeningWhitelists) (models.ScreeningWhitelist, error) {
	return models.ScreeningWhitelist{
		Id:             db.Id,
		OrgId:          db.OrgId.String(),
		CounterpartyId: db.CounterpartyId,
		EntityId:       db.EntityId,
		WhitelistedBy:  db.WhitelistedBy,
		CreatedAt:      db.CreatedAt,
	}, nil
}
