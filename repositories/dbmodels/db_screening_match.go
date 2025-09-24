package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_SCREENING_MATCHES = "screening_matches"

var SelectScreeningMatchesColumn = utils.ColumnList[DBScreeningMatch]()

type DBScreeningMatch struct {
	Id                   string          `db:"id"`
	ScreeningId          string          `db:"screening_id"`
	OpenSanctionEntityId string          `db:"opensanction_entity_id"`
	Status               string          `db:"status"`
	QueryIds             []string        `db:"query_ids"`
	CounterpartyId       *string         `db:"counterparty_id"`
	Payload              json.RawMessage `db:"payload"`
	Enriched             bool            `db:"enriched"`
	ReviewedBy           *string         `db:"reviewed_by"`
	CreatedAt            time.Time       `db:"created_at"`
	UpdatedAt            time.Time       `db:"updated_at"`

	Comments []DBScreeningMatchComment `db:"-"`
}

func AdaptScreeningMatch(dto DBScreeningMatch) (models.ScreeningMatch, error) {
	match := models.ScreeningMatch{
		Id:                           dto.Id,
		ScreeningId:                  dto.ScreeningId,
		EntityId:                     dto.OpenSanctionEntityId,
		Status:                       models.ScreeningMatchStatusFrom(dto.Status),
		ReviewedBy:                   dto.ReviewedBy,
		QueryIds:                     dto.QueryIds,
		UniqueCounterpartyIdentifier: dto.CounterpartyId,
		Payload:                      dto.Payload,
		Enriched:                     dto.Enriched,
	}

	return match, nil
}
