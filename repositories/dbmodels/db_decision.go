package dbmodels

import (
	"encoding/json"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

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
	ScheduledExecutionId string      `db:"scheduled_execution_id"`
}

const TABLE_DECISION = "decisions"

var SelectDecisionColumn = utils.ColumnList[DbDecision]()

func AdaptDecision(db DbDecision) models.Decision {

	triggerObject := make(map[string]any)
	err := json.Unmarshal(db.TriggerObjectRaw, &triggerObject)
	if err != nil {
		panic(fmt.Errorf("Can't decode %w decision's trigger object", err))
	}

	return models.Decision{
		DecisionId: db.ID,
		// OrgID
		CreatedAt:           db.CreatedAt,
		PayloadForArchive:   models.PayloadForArchive{TableName: db.TriggerObjectType, Data: triggerObject},
		Outcome:             models.OutcomeFrom(db.Outcome),
		ScenarioId:          db.ScenarioID,
		ScenarioName:        db.ScenarioName,
		ScenarioDescription: db.ScenarioDescription,
		ScenarioVersion:     db.ScenarioVersion,
		// RuleExecutions
		Score:                db.Score,
		DecisionError:        models.DecisionError(db.ErrorCode),
		ScheduledExecutionId: db.ScheduledExecutionId,
	}
}
