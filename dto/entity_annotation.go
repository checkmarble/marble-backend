package dto

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type EntityAnnotationDto struct {
	Id         string          `json:"id"`
	Type       string          `json:"type"`
	Payload    json.RawMessage `json:"payload"`
	AttachedBy *string         `json:"attached_by,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

type PostEntityAnnotationDto struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type PostEntityFileAnnotationDto struct {
	Caption string                 `form:"caption"`
	Files   []multipart.FileHeader `form:"files[]"`
}

func AdaptEntityAnnotation(model models.EntityAnnotation) (EntityAnnotationDto, error) {
	var userId *string
	if model.AttachedBy != nil {
		userId = utils.Ptr(string(*model.AttachedBy))
	}

	payload, err := AdaptEntityAnnotationPayload(model)
	if err != nil {
		return EntityAnnotationDto{}, err
	}

	return EntityAnnotationDto{
		Id:         model.Id,
		Type:       model.AnnotationType.String(),
		Payload:    model.Payload,
		AttachedBy: userId,
		CreatedAt:  model.CreatedAt,
	}

	return
}

func DecodeEntityAnnotationPayload(kind models.EntityAnnotationType, payload json.RawMessage) (out models.EntityAnnotationPayload, err error) {
	switch kind {
	case models.EntityAnnotationComment:
		var o models.EntityAnnotationCommentPayload

		err = json.Unmarshal(payload, &o)
		out = o

	case models.EntityAnnotationFile:
		var o models.EntityAnnotationFilePayload

		err = json.Unmarshal(payload, &o)
		out = o

	case models.EntityAnnotationTag:
		var comment models.EntityAnnotationCommentPayload
		err := json.Unmarshal(payload, &comment)
		return comment, err

	default:
		return nil, fmt.Errorf("invalid annotation type")
	}

	return
}
