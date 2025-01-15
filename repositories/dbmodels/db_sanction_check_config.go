package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_SANCTION_CHECK_CONFIGS = "sanction_check_configs"

type DBSanctionCheckConfigs struct {
	Id                  string    `db:"id"`
	ScenarioIterationId string    `db:"scenario_iteration_id"`
	Enabled             bool      `db:"enabled"`
	UpdatedAt           time.Time `db:"updated_at"`
}

var SanctionCheckConfigColumnList = utils.ColumnList[DBSanctionCheckConfigs]()

func AdaptSanctionCheckConfig(db DBSanctionCheckConfigs) (models.SanctionCheckConfig, error) {
	scc := models.SanctionCheckConfig{
		Enabled: db.Enabled,
	}

	return scc, nil
}
