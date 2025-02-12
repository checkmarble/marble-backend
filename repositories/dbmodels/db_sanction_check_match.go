package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_SANCTION_CHECK_MATCHES = "sanction_check_matches"

var SelectSanctionCheckMatchesColumn = utils.ColumnList[DBSanctionCheckMatch]()

type DBSanctionCheckMatch struct {
	Id                   string          `db:"id"`
	SanctionCheckId      string          `db:"sanction_check_id"`
	OpenSanctionEntityId string          `db:"opensanction_entity_id"`
	Status               string          `db:"status"`
	QueryIds             []string        `db:"query_ids"`
	CounterpartyId       *string         `db:"counterparty_id"`
	Payload              json.RawMessage `db:"payload"`
	ReviewedBy           *string         `db:"reviewed_by"`
	CreatedAt            time.Time       `db:"created_at"`
	UpdatedAt            time.Time       `db:"updated_at"`

	Comments []DBSanctionCheckMatchComment `db:"-"`
}

func AdaptSanctionCheckMatch(dto DBSanctionCheckMatch) (models.SanctionCheckMatch, error) {
	match := models.SanctionCheckMatch{
		Id:                           dto.Id,
		SanctionCheckId:              dto.SanctionCheckId,
		EntityId:                     dto.OpenSanctionEntityId,
		Status:                       models.SanctionCheckMatchStatusFrom(dto.Status),
		ReviewedBy:                   dto.ReviewedBy,
		QueryIds:                     dto.QueryIds,
		UniqueCounterpartyIdentifier: dto.CounterpartyId,
		Payload:                      dto.Payload,
	}

	return match, nil
}
