package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBScenarioPublication struct {
	Id                  string    `db:"id"`
	Rank                int32     `db:"rank"`
	OrganizationId      string    `db:"org_id"`
	ScenarioId          string    `db:"scenario_id"`
	ScenarioIterationId string    `db:"scenario_iteration_id"`
	PublicationAction   string    `db:"publication_action"`
	TestMode            bool      `db:"test_mode"`
	CreatedAt           time.Time `db:"created_at"`
}

const TABLE_SCENARIOS_PUBLICATIONS = "scenario_publications"

var SelectScenarioPublicationColumns = utils.ColumnList[DBScenarioPublication]()

func AdaptScenarioPublication(dto DBScenarioPublication) (models.ScenarioPublication, error) {
	scenarioPublication := models.ScenarioPublication{
		Id:                  dto.Id,
		OrganizationId:      dto.OrganizationId,
		ScenarioId:          dto.ScenarioId,
		ScenarioIterationId: dto.ScenarioIterationId,
		Rank:                dto.Rank,
		CreatedAt:           dto.CreatedAt,
		TestMode:            dto.TestMode,
		PublicationAction:   models.PublicationActionFrom(dto.PublicationAction),
	}

	return scenarioPublication, nil
}

type PublishScenarioIterationInput struct {
	OrganizationId      string
	ScenarioIterationId string
	ScenarioId          string
	PublicationAction   models.PublicationAction
}
