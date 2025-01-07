package dbmodels

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/jackc/pgx/v5/pgtype"
)

type DBScenario struct {
	Id                         string      `db:"id"`
	CreatedAt                  time.Time   `db:"created_at"`
	DecisionToCaseInboxId      pgtype.Text `db:"decision_to_case_inbox_id"`
	DecisionToCaseOutcomes     []string    `db:"decision_to_case_outcomes"`
	DecisionToCaseWorkflowType string      `db:"decision_to_case_workflow_type"`
	DecisionToCaseNameTemplate []byte      `db:"decision_to_case_name_template"`
	DeletedAt                  pgtype.Time `db:"deleted_at"`
	Description                string      `db:"description"`
	LiveVersionID              pgtype.Text `db:"live_scenario_iteration_id"`
	Name                       string      `db:"name"`
	OrganizationId             string      `db:"org_id"`
	TriggerObjectType          string      `db:"trigger_object_type"`
}

const TABLE_SCENARIOS = "scenarios"

var SelectScenarioColumn = utils.ColumnList[DBScenario]()

func AdaptScenario(dto DBScenario) (models.Scenario, error) {
	scenario := models.Scenario{
		Id:        dto.Id,
		CreatedAt: dto.CreatedAt,
		DecisionToCaseOutcomes: pure_utils.Map(dto.DecisionToCaseOutcomes,
			func(s string) models.Outcome { return models.OutcomeFrom(s) }),
		DecisionToCaseWorkflowType: models.WorkflowType(dto.DecisionToCaseWorkflowType),
		Description:                dto.Description,
		Name:                       dto.Name,
		OrganizationId:             dto.OrganizationId,
		TriggerObjectType:          dto.TriggerObjectType,
	}
	if dto.DecisionToCaseInboxId.Valid {
		scenario.DecisionToCaseInboxId = &dto.DecisionToCaseInboxId.String
	}
	if dto.LiveVersionID.Valid {
		scenario.LiveVersionID = &dto.LiveVersionID.String
	}

	var err error
	scenario.DecisionToCaseNameTemplate, err =
		AdaptSerializedAstExpression(dto.DecisionToCaseNameTemplate)
	if err != nil {
		return scenario, fmt.Errorf("unable to unmarshal ast expression: %w", err)
	}

	return scenario, nil
}
