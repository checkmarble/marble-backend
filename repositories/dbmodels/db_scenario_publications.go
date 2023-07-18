package dbmodels

import (
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"time"
)

type DBScenarioPublication struct {
	ID                  string    `db:"id"`
	Rank                int32     `db:"rank"`
	OrgID               string    `db:"org_id"`
	ScenarioID          string    `db:"scenario_id"`
	ScenarioIterationID string    `db:"scenario_iteration_id"`
	PublicationAction   string    `db:"publication_action"`
	CreatedAt           time.Time `db:"created_at"`
}

const TABLE_SCENARIOS_PUBLICATIONS = "scenario_publications"

var SelectScenarioPublicationColumns = utils.ColumnList[DBScenarioPublication]()

func AdaptScenarioPublication(dto DBScenarioPublication) models.ScenarioPublication {
	scenarioPublication := models.ScenarioPublication{
		ID:                  dto.ID,
		OrgID:               dto.OrgID,
		ScenarioID:          dto.ScenarioID,
		ScenarioIterationID: dto.ScenarioIterationID,
		Rank:                dto.Rank,
		CreatedAt:           dto.CreatedAt,
		PublicationAction:   models.PublicationActionFrom(dto.PublicationAction),
	}

	return scenarioPublication
}

type PublishScenarioIterationInput struct {
	OrganizationId      string
	ScenarioIterationID string
	ScenarioId          string
	PublicationAction   models.PublicationAction
}
