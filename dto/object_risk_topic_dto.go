package dto

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type ObjectRiskTopicDto struct {
	Id         uuid.UUID `json:"id"`
	OrgId      uuid.UUID `json:"org_id"`
	ObjectType string    `json:"object_type"`
	ObjectId   string    `json:"object_id"`
	Topics     []string  `json:"topics"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func AdaptObjectRiskTopicDto(m models.ObjectRiskTopic) ObjectRiskTopicDto {
	return ObjectRiskTopicDto{
		Id:         m.Id,
		OrgId:      m.OrgId,
		ObjectType: m.ObjectType,
		ObjectId:   m.ObjectId,
		Topics:     pure_utils.Map(m.Topics, func(t models.RiskTopic) string { return t.String() }),
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}

type ObjectRiskTopicUpsertInputDto struct {
	ObjectType string   `json:"object_type" binding:"required"`
	ObjectId   string   `json:"object_id" binding:"required"`
	Topics     []string `json:"topics" binding:"required"`
	Reason     string   `json:"reason"`
	Url        string   `json:"url"`
}

func (d ObjectRiskTopicUpsertInputDto) Adapt(
	orgId uuid.UUID,
	userId uuid.UUID,
) (models.ObjectRiskTopicWithEventUpsert, error) {
	topics := make([]models.RiskTopic, 0, len(d.Topics))
	for _, t := range d.Topics {
		topic := models.RiskTopicFrom(t)
		if topic == models.RiskTopicUnknown {
			return models.ObjectRiskTopicWithEventUpsert{},
				errors.Wrap(models.BadParameterError, "invalid topic in upsert input")
		}
		topics = append(topics, topic)
	}

	return models.NewObjectRiskTopicWithEventFromManualUpsert(
		orgId,
		d.ObjectType,
		d.ObjectId,
		topics,
		userId,
		d.Reason,
		d.Url,
	), nil
}

type ObjectRiskTopicFilterDto struct {
	ObjectType string   `form:"object_type"`
	ObjectId   string   `form:"object_id"`
	Topics     []string `form:"topics"`
}

type ObjectRiskTopicEventDto struct {
	Id            uuid.UUID       `json:"id"`
	OrgId         uuid.UUID       `json:"org_id"`
	Topics        []string        `json:"topics"`
	SourceType    string          `json:"source_type"`
	SourceDetails json.RawMessage `json:"source_details,omitempty"`
	UserId        *uuid.UUID      `json:"user_id,omitempty"`
	ApiKeyId      *uuid.UUID      `json:"api_key_id,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

func AdaptObjectRiskTopicEventDto(m models.ObjectRiskTopicEvent) ObjectRiskTopicEventDto {
	var sourceDetails json.RawMessage
	if m.SourceDetails != nil {
		sourceDetails, _ = m.SourceDetails.ToJSON()
	}

	return ObjectRiskTopicEventDto{
		Id:            m.Id,
		OrgId:         m.OrgId,
		Topics:        pure_utils.Map(m.Topics, func(t models.RiskTopic) string { return t.String() }),
		SourceType:    m.SourceType.String(),
		SourceDetails: sourceDetails,
		UserId:        m.UserId,
		ApiKeyId:      m.ApiKeyId,
		CreatedAt:     m.CreatedAt,
	}
}

func (d ObjectRiskTopicFilterDto) Adapt(orgId uuid.UUID) (models.ObjectRiskTopicFilter, error) {
	filter := models.ObjectRiskTopicFilter{
		OrgId: orgId,
	}

	if d.ObjectType != "" {
		filter.ObjectType = &d.ObjectType
	}
	if d.ObjectId != "" {
		filter.ObjectId = &d.ObjectId
	}
	if len(d.Topics) > 0 {
		topics := make([]models.RiskTopic, 0, len(d.Topics))
		for _, t := range d.Topics {
			topic := models.RiskTopicFrom(t)
			if topic == models.RiskTopicUnknown {
				return models.ObjectRiskTopicFilter{},
					errors.Wrap(models.BadParameterError, "invalid topic in filter")
			}
			topics = append(topics, topic)
		}
		filter.Topics = topics
	}

	return filter, nil
}
