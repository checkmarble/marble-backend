package dbmodels

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const TABLE_DECISIONS = "decisions"

type DbDecision struct {
	Id                   uuid.UUID  `db:"id"`
	OrganizationId       uuid.UUID  `db:"org_id"`
	CaseId               *string    `db:"case_id"`
	CreatedAt            time.Time  `db:"created_at"`
	Outcome              string     `db:"outcome"`
	PivotId              *uuid.UUID `db:"pivot_id"`
	PivotValue           *string    `db:"pivot_value"`
	ReviewStatus         *string    `db:"review_status"`
	ScenarioId           uuid.UUID  `db:"scenario_id"`
	ScenarioIterationId  uuid.UUID  `db:"scenario_iteration_id"`
	ScenarioName         string     `db:"scenario_name"`
	ScenarioDescription  string     `db:"scenario_description"`
	ScenarioVersion      int        `db:"scenario_version"`
	ScheduledExecutionId *string    `db:"scheduled_execution_id"`
	Score                int        `db:"score"`
	TriggerObjectRaw     []byte     `db:"trigger_object"`
	TriggerObjectType    string     `db:"trigger_object_type"`
}

type DbJoinDecisionAndCase struct {
	DbDecision
	DBCase
}

type DbDecisionsByOutcome struct {
	Version string `db:"scenario_version"`
	Outcome string `db:"outcome"`
	Score   int    `db:"score"`
	Total   int    `db:"total"`
}

type DBPaginatedDecisions struct {
	DbDecision
	DBCase
	RankNumber int
}

var SelectDecisionColumn = utils.ColumnList[DbDecision]()

func AdaptDecision(db DbDecision, decisionCase *models.Case) models.Decision {
	triggerObject := make(map[string]any)
	err := json.Unmarshal(db.TriggerObjectRaw, &triggerObject)
	if err != nil {
		panic(fmt.Errorf("can't decode %w decision's trigger object", err))
	}

	return models.Decision{
		DecisionId:           db.Id,
		OrganizationId:       db.OrganizationId,
		Case:                 decisionCase,
		CreatedAt:            db.CreatedAt,
		ClientObject:         models.ClientObject{TableName: db.TriggerObjectType, Data: triggerObject},
		Outcome:              models.OutcomeFrom(db.Outcome),
		PivotId:              db.PivotId,
		PivotValue:           db.PivotValue,
		ReviewStatus:         db.ReviewStatus,
		ScenarioId:           db.ScenarioId,
		ScenarioIterationId:  db.ScenarioIterationId,
		ScenarioName:         db.ScenarioName,
		ScenarioDescription:  db.ScenarioDescription,
		ScenarioVersion:      db.ScenarioVersion,
		Score:                db.Score,
		ScheduledExecutionId: db.ScheduledExecutionId,
	}
}

func AdaptDecisionCore(db DbDecision) models.DecisionCore {
	return models.DecisionCore{
		DecisionId:     db.Id,
		OrganizationId: db.OrganizationId,
		CreatedAt:      db.CreatedAt,
		Score:          db.Score,
	}
}

func AdaptDecisionByOutcome(db DbDecisionsByOutcome) models.DecisionsByVersionByOutcome {
	return models.DecisionsByVersionByOutcome{
		Version: db.Version,
		Outcome: db.Outcome,
		Score:   db.Score,
		Count:   db.Total,
	}
}

func AdaptDecisionWithRuleExecutions(db DbDecision, ruleExecutions []models.RuleExecution,
	decisionCase *models.Case,
) models.DecisionWithRuleExecutions {
	decision := AdaptDecision(db, decisionCase)
	return models.DecisionWithRuleExecutions{Decision: decision, RuleExecutions: ruleExecutions}
}

func AdaptDecisionWithRank(db DbDecision, decisionCase *models.Case, rankNumber int) models.DecisionWithRank {
	decision := AdaptDecision(db, decisionCase)
	return models.DecisionWithRank{
		Decision:   decision,
		RankNumber: rankNumber,
	}
}
