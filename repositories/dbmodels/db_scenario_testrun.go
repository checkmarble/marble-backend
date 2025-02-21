package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBScenarioTestRun struct {
	Id                      string    `db:"id"`
	ScenarioIterationId     string    `db:"scenario_iteration_id"`
	LiveScenarioIterationId string    `db:"live_scenario_iteration_id"`
	CreatedAt               time.Time `db:"created_at"`
	ExpiresAt               time.Time `db:"expires_at"`
	Status                  string    `db:"status"`
	Summarized              bool      `db:"summarized"`
}

type DBScenarioTestRunWitInfo struct {
	DBScenarioTestRun
	OrgId      string `db:"org_id"`
	ScenarioId string `db:"scenario_id"`
}

type DBScenarioTestRunWithSummary struct {
	DBScenarioTestRun

	Summary []DbScenarioTestRunSummary `db:"summaries"`
}

const TABLE_SCENARIO_TESTRUN = "scenario_test_run"

var SelectScenarioTestRunColumns = utils.ColumnList[DBScenarioTestRun]()

func AdaptScenarioTestrun(db DBScenarioTestRun) (models.ScenarioTestRun, error) {
	return models.ScenarioTestRun{
		ScenarioIterationId:     db.ScenarioIterationId,
		Id:                      db.Id,
		ScenarioLiveIterationId: db.LiveScenarioIterationId,
		CreatedAt:               db.CreatedAt,
		ExpiresAt:               db.ExpiresAt,
		Status:                  models.ScenarioTestStatusFrom(db.Status),
		Summarized:              db.Summarized,
	}, nil
}

func AdaptScenarioTestrunWithInfo(db DBScenarioTestRunWitInfo) (models.ScenarioTestRun, error) {
	return models.ScenarioTestRun{
		ScenarioIterationId:     db.ScenarioIterationId,
		Id:                      db.Id,
		ScenarioLiveIterationId: db.LiveScenarioIterationId,
		CreatedAt:               db.CreatedAt,
		ExpiresAt:               db.ExpiresAt,
		OrganizationId:          db.OrgId,
		ScenarioId:              db.ScenarioId,
		Status:                  models.ScenarioTestStatusFrom(db.Status),
		Summarized:              db.Summarized,
	}, nil
}

func AdaptScenarioTestrunWithSummary(db DBScenarioTestRunWithSummary) (models.ScenarioTestRunWithSummary, error) {
	summaries := make([]models.ScenarioTestRunSummary, len(db.Summary))

	for idx, s := range db.Summary {
		summary, err := AdaptScenarioTestRunSummary(s)
		if err != nil {
			return models.ScenarioTestRunWithSummary{}, nil
		}
		summaries[idx] = summary
	}

	testRun, err := AdaptScenarioTestrun(db.DBScenarioTestRun)
	if err != nil {
		return models.ScenarioTestRunWithSummary{}, err
	}

	return models.ScenarioTestRunWithSummary{
		ScenarioTestRun: testRun,
		Summary:         summaries,
	}, nil
}
