package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/utils"
)

type DBScenarioTestRun struct {
	Id                  string        `db:"id"`
	ScenarioIterationId string        `db:"scenario_iteration_id"`
	CreatedAt           time.Time     `db:"created_at"`
	Period              time.Duration `db:"period"`
	Status              string        `db:"status"`
}

const TABLE_SCENARIO_TESTRUN = "scenario_testrun"

var SelectScenarioTestRunColumns = utils.ColumnList[DBScenarioTestRun]()
