package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DBObjectRiskTopic struct {
	Id         uuid.UUID `db:"id"`
	OrgId      uuid.UUID `db:"org_id"`
	ObjectType string    `db:"object_type"`
	ObjectId   string    `db:"object_id"`
	Topics     []string  `db:"topics"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

const TABLE_OBJECT_RISK_TOPICS = "object_risk_topics"

var SelectObjectRiskTopicColumn = utils.ColumnList[DBObjectRiskTopic]()

func AdaptObjectRiskTopic(db DBObjectRiskTopic) (models.ObjectRiskTopic, error) {
	topics := make([]models.RiskTopic, len(db.Topics))
	for i, t := range db.Topics {
		topics[i] = models.RiskTopicFrom(t)
	}

	return models.ObjectRiskTopic{
		Id:         db.Id,
		OrgId:      db.OrgId,
		ObjectType: db.ObjectType,
		ObjectId:   db.ObjectId,
		Topics:     topics,
		CreatedAt:  db.CreatedAt,
		UpdatedAt:  db.UpdatedAt,
	}, nil
}
