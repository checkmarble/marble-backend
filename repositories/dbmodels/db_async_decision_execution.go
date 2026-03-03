package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DBAsyncDecisionExecution struct {
	Id            uuid.UUID       `db:"id"`
	OrgId         uuid.UUID       `db:"org_id"`
	ObjectType    string          `db:"object_type"`
	TriggerObject json.RawMessage `db:"trigger_object"`

	// Created in advance to avoid a future migration. Unused for now.
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

	return models.AsyncDecisionExecution{
		Id:            db.Id,
		OrgId:         db.OrgId,
		ObjectType:    db.ObjectType,
		TriggerObject: db.TriggerObject,
		ShouldIngest:  db.ShouldIngest,
		Status:        models.AsyncDecisionExecutionStatusFromString(db.Status),
		DecisionIds:   decisionIds,
		ErrorMessage:  db.ErrorMessage,
		CreatedAt:     db.CreatedAt,
		UpdatedAt:     db.UpdatedAt,
	}, nil
}
