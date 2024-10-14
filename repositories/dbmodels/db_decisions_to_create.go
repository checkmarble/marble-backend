package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DecisionToCreate struct {
	Id                   string    `db:"id"`
	ScheduledExecutionId string    `db:"scheduled_execution_id"`
	ObjectId             string    `db:"object_id"`
	Status               string    `db:"status"`
	CreatedAt            time.Time `db:"created_at"`
	UpdateAt             time.Time `db:"updated_at"`
}

const TABLE_DECISIONS_TO_CREATE = "decisions_to_create"

var DecisionToCreateFields = utils.ColumnList[DecisionToCreate]()

func AdaptDecisionToCrate(db DecisionToCreate) (models.DecisionToCreate, error) {
	return models.DecisionToCreate{
		Id:                   db.Id,
		ScheduledExecutionId: db.ScheduledExecutionId,
		ObjectId:             db.ObjectId,
		Status:               models.DecisionToCreateStatus(db.Status),
		CreatedAt:            db.CreatedAt,
		UpdateAt:             db.UpdateAt,
	}, nil
}
