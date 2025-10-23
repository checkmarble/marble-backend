package models

import (
	"time"

	"github.com/checkmarble/marble-backend/pure_utils"
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
	decisionId := uuid.Must(uuid.NewV7())
	return PhantomDecision{
		PhantomDecisionId:   decisionId.String(),
		CreatedAt:           time.Now(),
		OrganizationId:      scenarioExecution.OrganizationId.String(),
		Outcome:             scenarioExecution.Outcome,
		ScenarioId:          scenarioExecution.ScenarioId.String(),
		ScenarioIterationId: scenarioExecution.ScenarioIterationId.String(),
		Score:               scenarioExecution.Score,
		RuleExecutions:      scenarioExecution.RuleExecutions,
		ScreeningExecutions: pure_utils.Map(scenarioExecution.ScreeningExecutions,
			MergeScreeningExecWithDefaults(decisionId, scenarioExecution.OrganizationId)),
	}
}
