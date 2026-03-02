package dto

import (
	"encoding/json"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi/types"
)

type RecordAnnotationDto struct {
	Id             string         `json:"id"`
	ObjectType     string         `json:"object_type"`
	ObjectId       string         `json:"object_id"`
	AnnotationType string         `json:"annotation_type"`
	Payload        any            `json:"payload"`
	CreatedAt      types.DateTime `json:"created_at"`
}

type RecordCommentPayload struct {
	Text string `json:"text"`
}

type RecordTagPayload struct {
	TagId string `json:"tag_id"`
}

type RecordFilePayload struct {
	Caption string                 `json:"caption"`
	Files   []RecordFilePayloadFile `json:"files"`
}

type RecordFilePayloadFile struct {
	Id       string `json:"id"`
	Filename string `json:"filename"`
}

type RecordRiskTagPayload struct {
	Tag                   string `json:"tag"`
	Reason                string `json:"reason,omitempty"`
	Url                   string `json:"url,omitempty"`
	ContinuousScreeningId string `json:"continuous_screening_id,omitempty"`
	OpenSanctionsEntityId string `json:"opensanctions_entity_id,omitempty"` //nolint: tagliatelle
}

func AdaptRecordAnnotationDto(m models.EntityAnnotation) (RecordAnnotationDto, error) {
	payload, err := adaptRecordAnnotationPayload(m)
	if err != nil {
		return RecordAnnotationDto{}, err
	}
	return RecordAnnotationDto{
		Id:             m.Id,
		ObjectType:     m.ObjectType,
		ObjectId:       m.ObjectId,
		AnnotationType: m.AnnotationType.String(),
		Payload:        payload,
		CreatedAt:      types.DateTime(m.CreatedAt),
	}, nil
}

func adaptRecordAnnotationPayload(m models.EntityAnnotation) (any, error) {
	switch m.AnnotationType {
	case models.EntityAnnotationComment:
		var p RecordCommentPayload
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			return nil, err
		}
		return p, nil

	case models.EntityAnnotationTag:
		var p RecordTagPayload
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			return nil, err
		}
		return p, nil

	case models.EntityAnnotationFile:
		var p RecordFilePayload
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			return nil, err
		}
		return p, nil

	case models.EntityAnnotationRiskTag:
		var p RecordRiskTagPayload
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			return nil, err
		}
		return p, nil

	default:
		// Should never happen
		return nil, fmt.Errorf("invalid annotation type: %s", m.AnnotationType)
	}
}

type RecordFileUrl struct {
	Url string `json:"url"`
}
