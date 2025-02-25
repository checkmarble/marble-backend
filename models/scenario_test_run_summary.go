package models

import "time"

type ScenarioTestRunSummary struct {
	Id           string
	RuleName     *string
	RuleStableId *string
	TestRunId    string
	Version      int
	Watermark    time.Time
	Outcome      string
	Total        int
}
