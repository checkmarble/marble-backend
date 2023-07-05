package dbmodels

import (
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type DBScenario struct {
	ID                string      `db:"id"`
	OrgID             string      `db:"org_id"`
	Name              string      `db:"name"`
	Description       string      `db:"description"`
	TriggerObjectType string      `db:"trigger_object_type"`
	CreatedAt         time.Time   `db:"created_at"`
	LiveVersionID     pgtype.Text `db:"live_scenario_iteration_id"`
	DeletedAt         pgtype.Time `db:"deleted_at"`
}

const TABLE_SCENARIOS = "scenarios"

var SelectScenarioColumn = utils.ColumnList[DBScenario]()

func AdaptScenario(dto DBScenario) models.Scenario {
	scenario := models.Scenario{
		ID:                dto.ID,
		OrganizationID:    dto.OrgID,
		Name:              dto.Name,
		Description:       dto.Description,
		TriggerObjectType: dto.TriggerObjectType,
		CreatedAt:         dto.CreatedAt,
	}
	if dto.LiveVersionID.Valid {
		scenario.LiveVersionID = &dto.LiveVersionID.String
	}
	return scenario
}
