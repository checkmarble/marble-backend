package dto

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type ObjectMetadataDto struct {
	Id           uuid.UUID       `json:"id"`
	OrgId        uuid.UUID       `json:"org_id"`
	ObjectType   string          `json:"object_type"`
	ObjectId     string          `json:"object_id"`
	MetadataType string          `json:"metadata_type"`
	Metadata     json.RawMessage `json:"metadata"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

func AdaptObjectMetadataDto(m models.ObjectMetadata) (ObjectMetadataDto, error) {
	var metadata json.RawMessage
	if m.Metadata != nil {
		var err error
		metadata, err = m.Metadata.ToJSON()
		if err != nil {
			return ObjectMetadataDto{}, errors.Wrap(err, "failed to serialize metadata")
		}
	}

	return ObjectMetadataDto{
		Id:           m.Id,
		OrgId:        m.OrgId,
		ObjectType:   m.ObjectType,
		ObjectId:     m.ObjectId,
		MetadataType: m.MetadataType.String(),
		Metadata:     metadata,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}, nil
}

type ObjectRiskTopicUpsertInputDto struct {
	Topics []string `json:"topics" binding:"required"`
	Reason string   `json:"reason"`
	Url    string   `json:"url"`
}

func (d ObjectRiskTopicUpsertInputDto) Adapt(
	orgId uuid.UUID,
	objectType string,
	objectId string,
) (models.ObjectRiskTopicUpsert, error) {
	topics := make([]models.RiskTopic, 0, len(d.Topics))
	for _, t := range d.Topics {
		topic := models.RiskTopicFrom(t)
		if topic == models.RiskTopicUnknown {
			return models.ObjectRiskTopicUpsert{},
				errors.Wrap(models.BadParameterError, "invalid topic in upsert input")
		}
		topics = append(topics, topic)
	}

	return models.NewObjectRiskTopicFromManualUpsert(
		orgId,
		objectType,
		objectId,
		topics,
		d.Reason,
		d.Url,
	), nil
}

// AdaptRiskTopicAnnotationToObjectMetadataDto adapts an EntityAnnotation (risk_topic type)
// to ObjectMetadataDto for backwards compatibility with the object-metadata API
func AdaptRiskTopicAnnotationToObjectMetadataDto(a models.EntityAnnotation) (ObjectMetadataDto, error) {
	annotationId, err := uuid.Parse(a.Id)
	if err != nil {
		return ObjectMetadataDto{}, errors.Wrap(err, "failed to parse annotation ID")
	}

	return ObjectMetadataDto{
		Id:           annotationId,
		OrgId:        a.OrgId,
		ObjectType:   a.ObjectType,
		ObjectId:     a.ObjectId,
		MetadataType: models.MetadataTypeRiskTopics.String(),
		Metadata:     a.Payload,
		CreatedAt:    a.CreatedAt,
		UpdatedAt:    a.UpdatedAt,
	}, nil
}
