package dbmodels

import (
	"encoding/json"
	"marble/marble-backend/models"
	"marble/marble-backend/pg_repository"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type DBDecision struct {
	ID                  string      `db:"id"`
	OrgID               string      `db:"org_id"`
	CreatedAt           time.Time   `db:"created_at"`
	Outcome             string      `db:"outcome"`
	ScenarioID          string      `db:"scenario_id"`
	ScenarioName        string      `db:"scenario_name"`
	ScenarioDescription string      `db:"scenario_description"`
	ScenarioVersion     int         `db:"scenario_version"`
	Score               int         `db:"score"`
	ErrorCode           int         `db:"error_code"`
	DeletedAt           pgtype.Time `db:"deleted_at"`
	TriggerObjectRaw    []byte      `db:"trigger_object"`
	TriggerObjectType   string      `db:"trigger_object_type"`
}

const TABLE_DECISIONS = "decisions"
const TABLE_DECISION_RULES = "decision_rules"

var ColumnsSelectDecision = pg_repository.ColumnList[DBDecision]()

func AdaptDecision(db DBDecision) models.Decision {
	triggerObject := make(map[string]interface{})
	err := json.Unmarshal(db.TriggerObjectRaw, &triggerObject)
	if err != nil {
		panic(err)
	}

	return models.Decision{
		ID:                  db.ID,
		CreatedAt:           db.CreatedAt,
		Outcome:             models.OutcomeFrom(db.Outcome),
		ScenarioID:          db.ScenarioID,
		ScenarioName:        db.ScenarioName,
		ScenarioDescription: db.ScenarioDescription,
		ScenarioVersion:     db.ScenarioVersion,
		RuleExecutions:      []models.RuleExecution{},
		Score:               db.Score,
		DecisionError:       models.DecisionError(db.ErrorCode),
		ClientObject:        models.ClientObject{TableName: models.TableName(db.TriggerObjectType), Data: triggerObject},
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
