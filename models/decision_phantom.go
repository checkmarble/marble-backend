package models

import (
	"time"

	"github.com/google/uuid"
)

type CreatePhantomDecisionInput struct {
	OrganizationId     string
	Scenario           Scenario
	ClientObject       ClientObject
	Pivot              *Pivot
	TriggerObjectTable string
}

type PhantomDecision struct {
	PhantomDecisionId   string
	CreatedAt           time.Time
	OrganizationId      string
	Outcome             Outcome
	ScenarioId          string
	ScenarioIterationId string
	Score               int
	RuleExecutions      []RuleExecution
	ScreeningExecutions []ScreeningWithMatches
}

func AdaptScenarExecToPhantomDecision(scenarioExecution ScenarioExecution) PhantomDecision {
	return PhantomDecision{
		PhantomDecisionId:   uuid.Must(uuid.NewV7()).String(),
		CreatedAt:           time.Now(),
		OrganizationId:      scenarioExecution.OrganizationId.String(),
		Outcome:             scenarioExecution.Outcome,
		ScenarioId:          scenarioExecution.ScenarioId.String(),
		ScenarioIterationId: scenarioExecution.ScenarioIterationId.String(),
		Score:               scenarioExecution.Score,
		RuleExecutions:      scenarioExecution.RuleExecutions,
		ScreeningExecutions: scenarioExecution.ScreeningExecutions,
	}
}
