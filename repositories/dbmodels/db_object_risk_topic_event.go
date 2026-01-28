package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DBObjectRiskTopicEvent struct {
	Id                 uuid.UUID       `db:"id"`
	OrgId              uuid.UUID       `db:"org_id"`
	ObjectRiskTopicsId uuid.UUID       `db:"object_risk_topics_id"`
	Topics             []string        `db:"topics"`
	SourceType         string          `db:"source_type"`
	SourceDetails      json.RawMessage `db:"source_details"`
	UserId             *uuid.UUID      `db:"user_id"`
	ApiKeyId           *uuid.UUID      `db:"api_key_id"`
	CreatedAt          time.Time       `db:"created_at"`
}

const TABLE_OBJECT_RISK_TOPIC_EVENTS = "object_risk_topic_events"

var SelectObjectRiskTopicEventColumn = utils.ColumnList[DBObjectRiskTopicEvent]()

func AdaptObjectRiskTopicEvent(db DBObjectRiskTopicEvent) (models.ObjectRiskTopicEvent, error) {
	topics := make([]models.RiskTopic, len(db.Topics))
	for i, t := range db.Topics {
		topics[i] = models.RiskTopicFrom(t)
	}

	sourceType := models.RiskTopicSourceTypeFrom(db.SourceType)
	sourceDetails, err := models.ParseSourceDetails(sourceType, db.SourceDetails)
	if err != nil {
		return models.ObjectRiskTopicEvent{}, err
	}

	event := models.ObjectRiskTopicEvent{
		Id:                 db.Id,
		OrgId:              db.OrgId,
		ObjectRiskTopicsId: db.ObjectRiskTopicsId,
		Topics:             topics,
		SourceType:         sourceType,
		SourceDetails:      sourceDetails,
		UserId:             db.UserId,
		ApiKeyId:           db.ApiKeyId,
		CreatedAt:          db.CreatedAt,
	}
	return event, nil
}
