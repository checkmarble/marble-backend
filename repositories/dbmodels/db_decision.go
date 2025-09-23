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

type DbCoreDecision struct {
	Id                   uuid.UUID      `db:"id"`
	OrganizationId       uuid.UUID      `db:"org_id"`
	CaseId               *string        `db:"case_id"`
	CreatedAt            time.Time      `db:"created_at"`
	Outcome              string         `db:"outcome"`
	PivotId              *uuid.UUID     `db:"pivot_id"`
	PivotValue           *string        `db:"pivot_value"`
	ReviewStatus         *string        `db:"review_status"`
	ScenarioId           uuid.UUID      `db:"scenario_id"`
	ScenarioIterationId  uuid.UUID      `db:"scenario_iteration_id"`
	ScheduledExecutionId *string        `db:"scheduled_execution_id"`
	Score                int            `db:"score"`
	TriggerObjectRaw     []byte         `db:"trigger_object"`
	TriggerObjectType    string         `db:"trigger_object_type"`
	AnalyticsFields      map[string]any `db:"analytics_fields"`
}

type DbCoreDecisionWithScenario struct {
	DbCoreDecision
	ScenarioName        string `db:"scenario_name"`
	ScenarioDescription string `db:"scenario_description"`
	ScenarioVersion     int    `db:"scenario_version"`
}

type DbCoreDecisionWithCaseAndScenario struct {
	DbCoreDecision
	DBCase
	ScenarioName        string `db:"scenario_name"`
	ScenarioDescription string `db:"scenario_description"`
	ScenarioVersion     int    `db:"scenario_version"`
}

type DbDecisionsByOutcome struct {
	Version string `db:"scenario_version"`
	Outcome string `db:"outcome"`
	Score   int    `db:"score"`
	Total   int    `db:"total"`
}

var SelectCoreDecisionColumn = utils.ColumnList[DbCoreDecision]()

func adaptCoreDecision(db DbCoreDecision, decisionCase *models.Case) models.Decision {
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
		Score:                db.Score,
		ScheduledExecutionId: db.ScheduledExecutionId,
	}
}

func AdaptDecision(db DbCoreDecisionWithScenario) (models.Decision, error) {
	decision := adaptCoreDecision(db.DbCoreDecision, nil)
	decision.ScenarioName = db.ScenarioName
	decision.ScenarioDescription = db.ScenarioDescription
	decision.ScenarioVersion = db.ScenarioVersion
	return decision, nil
}

func AdaptDecisionWithCase(db DbCoreDecisionWithCaseAndScenario) (models.Decision, error) {
	var decisionCase *models.Case
	if db.DBCase.Id.Valid {
		decisionCaseValue, err := AdaptCase(db.DBCase)
		if err != nil {
			return models.Decision{}, err
		}
		decisionCase = &decisionCaseValue
	}

	decision := adaptCoreDecision(db.DbCoreDecision, decisionCase)
	decision.ScenarioName = db.ScenarioName
	decision.ScenarioDescription = db.ScenarioDescription
	decision.ScenarioVersion = db.ScenarioVersion
	return decision, nil
}

func AdaptDecisionWithRuleExecutions(
	db DbCoreDecisionWithCaseAndScenario,
	ruleExecutions []models.RuleExecution,
) (models.DecisionWithRuleExecutions, error) {
	decision, err := AdaptDecisionWithCase(db)
	if err != nil {
		return models.DecisionWithRuleExecutions{}, err
	}
	return models.DecisionWithRuleExecutions{Decision: decision, RuleExecutions: ruleExecutions}, nil
}

func AdaptDecisionMetadata(db DbCoreDecision) models.DecisionMetadata {
	return models.DecisionMetadata{
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
