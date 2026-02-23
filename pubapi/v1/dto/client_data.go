package dto

import (
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi/types"
)

type ClientDataAnnotationDto struct {
	Id             string          `json:"id"`
	ObjectType     string          `json:"object_type"`
	ObjectId       string          `json:"object_id"`
	AnnotationType string          `json:"annotation_type"`
	Payload        json.RawMessage `json:"payload"`
	CreatedAt      types.DateTime  `json:"created_at"`
}

func AdaptClientDataAnnotationDto(m models.EntityAnnotation) ClientDataAnnotationDto {
	return ClientDataAnnotationDto{
		Id:             m.Id,
		ObjectType:     m.ObjectType,
		ObjectId:       m.ObjectId,
		AnnotationType: m.AnnotationType.String(),
		Payload:        m.Payload,
		CreatedAt:      types.DateTime(m.CreatedAt),
	}
}
