package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type DBAsyncDecisionExecution struct {
	Id            uuid.UUID       `db:"id"`
	OrgId         uuid.UUID       `db:"org_id"`
	ObjectType    string          `db:"object_type"`
	TriggerObject json.RawMessage `db:"trigger_object"`

	ScenarioId   *string     `db:"scenario_id"`
	ShouldIngest bool        `db:"should_ingest"`
	Status       string      `db:"status"`
	DecisionIds  []uuid.UUID `db:"decision_ids"`
	ErrorMessage *string     `db:"error_message"`
	CreatedAt    time.Time   `db:"created_at"`
	UpdatedAt    time.Time   `db:"updated_at"`
}

const TABLE_ASYNC_DECISION_EXECUTIONS = "async_decision_executions"

var SelectAsyncDecisionExecutionColumns = utils.ColumnList[DBAsyncDecisionExecution]()

func AdaptAsyncDecisionExecution(db DBAsyncDecisionExecution) (models.AsyncDecisionExecution, error) {
	decisionIds := make([]uuid.UUID, len(db.DecisionIds))
	copy(decisionIds, db.DecisionIds)

	status := models.AsyncDecisionExecutionStatusFromString(db.Status)
	if status == models.AsyncDecisionExecutionStatusUnknown {
		return models.AsyncDecisionExecution{},
			errors.Wrapf(nil, "unknown async decision execution status: %s", db.Status)
	}
	return models.AsyncDecisionExecution{
		Id:            db.Id,
		OrgId:         db.OrgId,
		ObjectType:    db.ObjectType,
		ScenarioId:    db.ScenarioId,
		TriggerObject: db.TriggerObject,
		ShouldIngest:  db.ShouldIngest,
		Status:        status,
		DecisionIds:   decisionIds,
		ErrorMessage:  db.ErrorMessage,
		CreatedAt:     db.CreatedAt,
		UpdatedAt:     db.UpdatedAt,
	}, nil
}
