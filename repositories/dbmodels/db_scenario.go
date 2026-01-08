package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgtype"
)

type DBScenario struct {
	Id                string      `db:"id"`
	CreatedAt         time.Time   `db:"created_at"`
	DeletedAt         pgtype.Time `db:"deleted_at"`
	Description       string      `db:"description"`
	LiveVersionID     pgtype.Text `db:"live_scenario_iteration_id"`
	Name              string      `db:"name"`
	OrganizationId    uuid.UUID   `db:"org_id"`
	TriggerObjectType string      `db:"trigger_object_type"`
}

const TABLE_SCENARIOS = "scenarios"

var SelectScenarioColumn = utils.ColumnList[DBScenario]()

func AdaptScenario(dto DBScenario) (models.Scenario, error) {
	scenario := models.Scenario{
		Id:                dto.Id,
		CreatedAt:         dto.CreatedAt,
		Description:       dto.Description,
		Name:              dto.Name,
		OrganizationId:    dto.OrganizationId,
		TriggerObjectType: dto.TriggerObjectType,
	}

	if dto.LiveVersionID.Valid {
		scenario.LiveVersionID = &dto.LiveVersionID.String
	}

	return scenario, nil
}

type DBScenarioRuleLatestVersion struct {
	Type         string `db:"type"`
	StableRuleId string `db:"stable_rule_id"`
	Name         string `db:"name"`
	Version      string `db:"version"`
}
