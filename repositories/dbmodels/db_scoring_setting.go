package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DbScoringSetting struct {
	Id    uuid.UUID `db:"id"`
	OrgId uuid.UUID `db:"org_id"`

	MaxScore int `db:"max_score"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

const TABLE_SCORING_SETTINGS = "scoring_settings"

var SelectScoringSettingsColumns = utils.ColumnList[DbScoringSetting]()

func AdaptScoringSetting(db DbScoringSetting) (models.ScoringSettings, error) {
	return models.ScoringSettings{
		Id:        db.Id,
		OrgId:     db.OrgId,
		MaxScore:  db.MaxScore,
		CreatedAt: db.CreatedAt,
		UpdatedAt: db.UpdatedAt,
	}, nil
}
