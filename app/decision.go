package app

import (
	"marble/marble-backend/models"
	"time"
)

type Decision struct {
	ID                  string
	CreatedAt           time.Time
	PayloadForArchive   models.PayloadForArchive
	Outcome             models.Outcome
	ScenarioID          string
	ScenarioName        string
	ScenarioDescription string
	ScenarioVersion     int
	RuleExecutions      []RuleExecution
	Score               int
	DecisionError       models.DecisionError
}
