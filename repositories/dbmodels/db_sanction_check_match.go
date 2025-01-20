package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_SANCTION_CHECK_MATCHES = "sanction_check_matches"

var SelectSanctionCheckMatchesColumn = utils.ColumnList[DBSanctionCheckMatch]()

type DBSanctionCheckMatch struct {
	Id                   string    `db:"id"`
	SanctionCheckId      string    `db:"sanction_check_id"`
	OpenSanctionEntityId string    `db:"opensanction_entity_id"`
	Status               string    `db:"status"`
	QueryIds             []string  `db:"query_ids"`
	Payload              []byte    `db:"payload"`
	ReviewedBy           *string   `db:"reviewed_by"`
	CreatedAt            time.Time `db:"created_at"`
	UpdatedAt            time.Time `db:"updated_at"`
}

func AdaptSanctionCheckMatch(dto DBSanctionCheckMatch) (models.SanctionCheckExecutionMatch, error) {
	match := models.SanctionCheckExecutionMatch{
		Id:       dto.Id,
		EntityId: dto.OpenSanctionEntityId,
	}

	return match, nil
}
