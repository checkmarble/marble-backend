package dto

import (
	"encoding/json"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi/types"
)

type ClientDataAnnotationDto struct {
	Id             string         `json:"id"`
	ObjectType     string         `json:"object_type"`
	ObjectId       string         `json:"object_id"`
	AnnotationType string         `json:"annotation_type"`
	Payload        any            `json:"payload"`
	CreatedAt      types.DateTime `json:"created_at"`
}

type ClientDataCommentPayload struct {
	Text string `json:"text"`
}

type ClientDataTagPayload struct {
	TagId string `json:"tag_id"`
}

type ClientDataFilePayload struct {
	Caption string                      `json:"caption"`
	Files   []ClientDataFilePayloadFile `json:"files"`
}

type ClientDataFilePayloadFile struct {
	Id       string `json:"id"`
	Filename string `json:"filename"`
}

type ClientDataRiskTagPayload struct {
	Tag                   string `json:"tag"`
	Reason                string `json:"reason,omitempty"`
	Url                   string `json:"url,omitempty"`
	ContinuousScreeningId string `json:"continuous_screening_id,omitempty"`
	OpenSanctionsEntityId string `json:"opensanctions_entity_id,omitempty"` //nolint: tagliatelle
}

func AdaptClientDataAnnotationDto(m models.EntityAnnotation) (ClientDataAnnotationDto, error) {
	payload, err := adaptClientDataAnnotationPayload(m)
	if err != nil {
		return ClientDataAnnotationDto{}, err
	}
	return ClientDataAnnotationDto{
		Id:             m.Id,
		ObjectType:     m.ObjectType,
		ObjectId:       m.ObjectId,
		AnnotationType: m.AnnotationType.String(),
		Payload:        payload,
		CreatedAt:      types.DateTime(m.CreatedAt),
	}, nil
}

func adaptClientDataAnnotationPayload(m models.EntityAnnotation) (any, error) {
	switch m.AnnotationType {
	case models.EntityAnnotationComment:
		var p ClientDataCommentPayload
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			return nil, err
		}
		return p, nil

	case models.EntityAnnotationTag:
		var p ClientDataTagPayload
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			return nil, err
		}
		return p, nil

	case models.EntityAnnotationFile:
		var p ClientDataFilePayload
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			return nil, err
		}
		return p, nil

	case models.EntityAnnotationRiskTag:
		var p ClientDataRiskTagPayload
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			return nil, err
		}
		return p, nil

	default:
		// Should never happen
		return nil, fmt.Errorf("invalid annotation type: %s", m.AnnotationType)
	}
}

type ClientDataFileUrl struct {
	Url string `json:"url"`
}
