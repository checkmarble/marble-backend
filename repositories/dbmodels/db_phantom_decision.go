package dbmodels

import "time"

const TABLE_PHANTOM_DECISIONS = "phantom_decisions"

type DbPhantomDecision struct {
	Id                  string    `db:"id"`
	OrganizationId      string    `db:"org_id"`
	CreatedAt           time.Time `db:"created_at"`
	Outcome             string    `db:"outcome"`
	ScenarioId          string    `db:"scenario_id"`
	Score               int       `db:"score"`
	ScenarioIterationId string    `db:"scenario_iteration_id"`
	TestRunId           string    `db:"test_run_id"`
	ScenarioVersion     string    `db:"scenario_version"`
}
