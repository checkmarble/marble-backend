package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const TABLE_CONTINUOUS_SCREENING_AUDIT = "_monitored_objects_audit"

type DBContinuousScreeningAudit struct {
	Id             uuid.UUID       `db:"id"`
	ObjectType     string          `db:"object_type"`
	ObjectId       string          `db:"object_id"`
	ConfigStableId uuid.UUID       `db:"config_stable_id"`
	Action         string          `db:"action"`
	UserId         *uuid.UUID      `db:"user_id"`
	ApiKeyId       *uuid.UUID      `db:"api_key_id"`
	Extra          json.RawMessage `db:"extra"`

	CreatedAt time.Time `db:"created_at"`
}

var ContinuousScreeningAuditColumnList = utils.ColumnList[DBContinuousScreeningAudit]()

func AdaptContinuousScreeningAudit(db DBContinuousScreeningAudit) (models.ContinuousScreeningAudit, error) {
	return models.ContinuousScreeningAudit{
		Id:             db.Id,
		ObjectType:     db.ObjectType,
		ObjectId:       db.ObjectId,
		ConfigStableId: db.ConfigStableId,
		Action:         models.ContinuousScreeningAuditActionFrom(db.Action),
		UserId:         db.UserId,
		ApiKeyId:       db.ApiKeyId,
		Extra:          db.Extra,
		CreatedAt:      db.CreatedAt,
	}, nil
}
