package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBScenarioTestRun struct {
	Id                  string    `db:"id"`
	ScenarioIterationId string    `db:"scenario_iteration_id"`
	CreatedAt           time.Time `db:"created_at"`
	ExpiresAt           time.Time `db:"expires_at"`
	Status              string    `db:"status"`
}

const TABLE_SCENARIO_TESTRUN = "scenario_test_run"

var SelectScenarioTestRunColumns = utils.ColumnList[DBScenarioTestRun]()

func AdaptScenarioTestrun(dto DBScenarioTestRun) (models.ScenarioTestRun, error) {
	return models.ScenarioTestRun{
		ScenarioIterationId: dto.ScenarioIterationId,
		ScenarioId:          dto.Id,
		CreatedAt:           dto.CreatedAt,
		ExpiresAt:           dto.ExpiresAt,
		Status:              models.ScenarioTestStatusFrom(dto.Status),
	}, nil
}
