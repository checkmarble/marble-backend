package dbmodels

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DbScenarioTestRunSummary struct {
	Id           string    `db:"id"`
	RuleName     string    `db:"rule_name"`
	RuleStableId string    `db:"rule_stable_id"`
	TestRunId    string    `db:"test_run_id"`
	Version      int       `db:"version"`
	Watermark    time.Time `db:"watermark"`
	Outcome      string    `db:"outcome"`
	Total        int       `db:"total"`
}

const TABLE_SCENARIO_TESTRUN_SUMMARIES = "scenario_test_run_summaries"

var SelectScenarioTestRunSummariesColumns = utils.ColumnList[DbScenarioTestRunSummary]()

func AdaptScenarioTestRunSummary(db DbScenarioTestRunSummary) (models.ScenarioTestRunSummary, error) {
	return models.ScenarioTestRunSummary{
		Id:           db.Id,
		RuleName:     db.RuleName,
		RuleStableId: db.RuleStableId,
		TestRunId:    db.TestRunId,
		Version:      db.Version,
		Watermark:    db.Watermark,
		Outcome:      db.Outcome,
		Total:        db.Total,
	}, nil
}

func AdaptToRuleExecutionStats(db DbScenarioTestRunSummary) (models.RuleExecutionStat, error) {
	return models.RuleExecutionStat{
		Version:      fmt.Sprintf("%d", db.Version),
		StableRuleId: &db.RuleStableId,
		Name:         db.RuleName,
		Outcome:      db.Outcome,
		Total:        db.Total,
	}, nil
}
