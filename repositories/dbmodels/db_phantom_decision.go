package dbmodels

import "time"

const TABLE_PHANTOM_DECISIONS = "phantom_decisions"

type DbPhantomDecision struct {
	Id                  string    `db:"id"`
	OrganizationId      string    `db:"org_id"`
	CreatedAt           time.Time `db:"created_at"`
	Outcome             string    `db:"outcome"`
	ScenarioId          string    `db:"scenario_id"`
	ScenarioVersion     int       `db:"scenario_version"`
	Score               int       `db:"score"`
	TriggerObjectRaw    []byte    `db:"trigger_object"`
	TriggerObjectType   string    `db:"trigger_object_type"`
	ScenarioIterationId string    `db:"scenario_iteration_id"`
	PivotId             *string   `db:"pivot_id"`
	PivotValue          *string   `db:"pivot_value"`
	TestRunId           string    `db:"test_run_id"`
}
