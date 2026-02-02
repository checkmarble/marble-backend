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

type ObjectMetadataFilterDto struct {
	ObjectType    string   `form:"object_type"`
	ObjectId      string   `form:"object_id"`
	MetadataTypes []string `form:"metadata_types"`
}

func (d ObjectMetadataFilterDto) Adapt(orgId uuid.UUID) (models.ObjectMetadataFilter, error) {
	filter := models.ObjectMetadataFilter{
		OrgId: orgId,
	}

	if d.ObjectType != "" {
		filter.ObjectType = &d.ObjectType
	}
	if d.ObjectId != "" {
		filter.ObjectIds = []string{d.ObjectId}
	}
	if len(d.MetadataTypes) > 0 {
		metadataTypes := make([]models.MetadataType, 0, len(d.MetadataTypes))
		for _, mt := range d.MetadataTypes {
			metadataType := models.MetadataTypeFrom(mt)
			if metadataType == models.MetadataTypeUnknown {
				return models.ObjectMetadataFilter{},
					errors.Wrap(models.BadParameterError, "invalid metadata_type: "+mt)
			}
			metadataTypes = append(metadataTypes, metadataType)
		}
		filter.MetadataTypes = metadataTypes
	}

	return filter, nil
}

type ObjectRiskTopicUpsertInputDto struct {
	Topics []string `json:"topics" binding:"required"`
	Reason string   `json:"reason"`
	Url    string   `json:"url"`
}

func (d ObjectRiskTopicUpsertInputDto) Adapt(
	orgId uuid.UUID,
	userId uuid.UUID,
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
		userId,
		d.Reason,
		d.Url,
	), nil
}
