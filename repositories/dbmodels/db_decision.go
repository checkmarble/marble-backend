package dbmodels

import (
	"encoding/json"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

const TABLE_DECISIONS = "decisions"
const TABLE_DECISION_RULES = "decision_rules"

// var ColumnsSelectDecision = utils.ColumnList[DBDecision]()

type DbDecision struct {
	ID                   string      `db:"id"`
	OrgID                string      `db:"org_id"`
	CreatedAt            time.Time   `db:"created_at"`
	Outcome              string      `db:"outcome"`
	ScenarioID           string      `db:"scenario_id"`
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

var SelectDecisionColumn = utils.ColumnList[DbDecision]()

func AdaptDecision(db DbDecision, ruleExecutions []models.RuleExecution) models.Decision {

	triggerObject := make(map[string]any)
	err := json.Unmarshal(db.TriggerObjectRaw, &triggerObject)
	if err != nil {
		panic(fmt.Errorf("can't decode %w decision's trigger object", err))
	}

	return models.Decision{
		DecisionId:           db.ID,
		OrganizationId:       db.OrgID,
		CreatedAt:            db.CreatedAt,
		ClientObject:         models.ClientObject{TableName: models.TableName(db.TriggerObjectType), Data: triggerObject},
		Outcome:              models.OutcomeFrom(db.Outcome),
		ScenarioId:           db.ScenarioID,
		ScenarioName:         db.ScenarioName,
		ScenarioDescription:  db.ScenarioDescription,
		ScenarioVersion:      db.ScenarioVersion,
		RuleExecutions:       ruleExecutions,
		Score:                db.Score,
		DecisionError:        models.DecisionError(db.ErrorCode),
		ScheduledExecutionId: db.ScheduledExecutionId,
	}
}

type DBDecisionRule struct {
	ID            string      `db:"id"`
	OrgID         string      `db:"org_id"`
	DecisionID    string      `db:"decision_id"`
	Name          string      `db:"name"`
	Description   string      `db:"description"`
	ScoreModifier int         `db:"score_modifier"`
	Result        bool        `db:"result"`
	ErrorCode     int         `db:"error_code"`
	DeletedAt     pgtype.Time `db:"deleted_at"`
}

func AdaptDecisionRule(db DBDecisionRule) models.RuleExecution {
	return models.RuleExecution{
		Rule: models.Rule{
			Name:        db.Name,
			Description: db.Description,
		},
		Result:              db.Result,
		ResultScoreModifier: db.ScoreModifier,
		Error:               models.RuleExecutionError(db.ErrorCode),
	}
}
