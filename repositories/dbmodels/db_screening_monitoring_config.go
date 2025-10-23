package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const TABLE_SCREENING_MONITORING_CONFIGS = "screening_monitoring_configs"

type DBScreeningMonitoringConfig struct {
	Id             uuid.UUID `db:"id"`
	OrgId          string    `db:"org_id"`
	Name           string    `db:"name"`
	Description    *string   `db:"description"`
	Datasets       []string  `db:"datasets"`
	MatchThreshold int       `db:"match_threshold"`
	MatchLimit     int       `db:"match_limit"`
	Enabled        bool      `db:"enabled"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

var ScreeningMonitoringConfigColumnList = utils.ColumnList[DBScreeningMonitoringConfig]()

func AdaptScreeningMonitoringConfig(db DBScreeningMonitoringConfig) (models.ScreeningMonitoringConfig, error) {
	return models.ScreeningMonitoringConfig{
		Id:             db.Id,
		OrgId:          db.OrgId,
		Name:           db.Name,
		Description:    db.Description,
		Datasets:       db.Datasets,
		MatchThreshold: db.MatchThreshold,
		MatchLimit:     db.MatchLimit,
		Enabled:        db.Enabled,
		CreatedAt:      db.CreatedAt,
		UpdatedAt:      db.UpdatedAt,
	}, nil
}
