package dbmodels

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/jackc/pgx/v5/pgtype"
)

const TABLE_DECISIONS = "decisions"

type DbDecision struct {
	Id                   string      `db:"id"`
	OrganizationId       string      `db:"org_id"`
	CaseId               *string     `db:"case_id"`
	CreatedAt            time.Time   `db:"created_at"`
	Outcome              string      `db:"outcome"`
	ScenarioId           string      `db:"scenario_id"`
	ScenarioIterationId  string      `db:"scenario_iteration_id"`
	ScenarioName         string      `db:"scenario_name"`
	ScenarioDescription  string      `db:"scenario_description"`
	ScenarioVersion      int         `db:"scenario_version"`
	Score                int         `db:"score"`
	ErrorCode            int         `db:"error_code"`
	DeletedAt            pgtype.Time `db:"deleted_at"`
	TriggerObjectRaw     []byte      `db:"trigger_object"`
	TriggerObjectType    string      `db:"trigger_object_type"`
	ScheduledExecutionId *string     `db:"scheduled_execution_id"`
}

type DbJoinDecisionAndCase struct {
	DbDecision
	DBCase
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
		ClientObject:         models.ClientObject{TableName: models.TableName(db.TriggerObjectType), Data: triggerObject},
		Outcome:              models.OutcomeFrom(db.Outcome),
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

func AdaptDecisionWithRuleExecutions(db DbDecision, ruleExecutions []models.RuleExecution,
	decisionCase *models.Case,
) models.DecisionWithRuleExecutions {
	decision := AdaptDecision(db, decisionCase)
	return models.DecisionWithRuleExecutions{Decision: decision, RuleExecutions: ruleExecutions}
}

func AdaptDecisionWithRank(db DbDecision, decisionCase *models.Case, rankNumber, total int) models.DecisionWithRank {
	decision := AdaptDecision(db, decisionCase)
	return models.DecisionWithRank{
		Decision:   decision,
		RankNumber: rankNumber,
		TotalCount: models.TotalCount{Value: total, IsMaxCount: total == models.COUNT_ROWS_LIMIT},
	}
}

type DBDecisionRule struct {
	Id             string      `db:"id"`
	OrganizationId string      `db:"org_id"`
	DecisionId     string      `db:"decision_id"`
	Name           string      `db:"name"`
	Description    string      `db:"description"`
	ScoreModifier  int         `db:"score_modifier"`
	Result         bool        `db:"result"`
	ErrorCode      int         `db:"error_code"`
	DeletedAt      pgtype.Time `db:"deleted_at"`
}
