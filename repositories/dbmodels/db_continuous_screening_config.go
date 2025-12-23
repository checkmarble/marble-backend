package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const TABLE_CONTINUOUS_SCREENING_CONFIGS = "continuous_screening_configs"

type DBContinuousScreeningConfig struct {
	Id             uuid.UUID `db:"id"`
	StableId       uuid.UUID `db:"stable_id"`
	OrgId          uuid.UUID `db:"org_id"`
	InboxId        uuid.UUID `db:"inbox_id"`
	Name           string    `db:"name"`
	Description    string    `db:"description"`
	Algorithm      string    `db:"algorithm"`
	ObjectTypes    []string  `db:"object_types"`
	Datasets       []string  `db:"datasets"`
	MatchThreshold int       `db:"match_threshold"`
	MatchLimit     int       `db:"match_limit"`
	Enabled        bool      `db:"enabled"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

var SelectContinuousScreeningConfigColumnList = utils.ColumnList[DBContinuousScreeningConfig]()

func AdaptContinuousScreeningConfig(db DBContinuousScreeningConfig) (models.ContinuousScreeningConfig, error) {
	return models.ContinuousScreeningConfig{
		Id:             db.Id,
		StableId:       db.StableId,
		OrgId:          db.OrgId,
		InboxId:        db.InboxId,
		Name:           db.Name,
		Description:    db.Description,
		Algorithm:      db.Algorithm,
		ObjectTypes:    db.ObjectTypes,
		Datasets:       db.Datasets,
		MatchThreshold: db.MatchThreshold,
		MatchLimit:     db.MatchLimit,
		Enabled:        db.Enabled,
		CreatedAt:      db.CreatedAt,
		UpdatedAt:      db.UpdatedAt,
	}, nil
}
