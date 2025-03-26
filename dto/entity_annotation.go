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
	Id          string    `json:"id"`
	Type        string    `json:"type"`
	Payload     any       `json:"payload"`
	AnnotatedBy *string   `json:"annotated_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
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
	if model.AnnotatedBy != nil {
		userId = utils.Ptr(string(*model.AnnotatedBy))
	}

	payload, err := AdaptEntityAnnotationPayload(model)
	if err != nil {
		return EntityAnnotationDto{}, err
	}

	return EntityAnnotationDto{
		Id:          model.Id,
		Type:        model.AnnotationType.String(),
		Payload:     payload,
		AnnotatedBy: userId,
		CreatedAt:   model.CreatedAt,
	}, nil
}

type returnEntityAnnotationComment struct {
	Text string `json:"text"`
}

type returnEntityAnnotationTag struct {
	Tag string `json:"tag"`
}

type returnEntityAnnotationFile struct {
	Caption string `json:"caption"`
	Files   []struct {
		Key      string `json:"key"`
		Filename string `json:"filename"`
	} `json:"files"`
}

func AdaptEntityAnnotationPayload(model models.EntityAnnotation) (out any, err error) {
	switch model.AnnotationType {
	case models.EntityAnnotationComment:
		var o returnEntityAnnotationComment

		err = json.Unmarshal(model.Payload, &o)
		out = o

	case models.EntityAnnotationTag:
		var o returnEntityAnnotationTag

		err = json.Unmarshal(model.Payload, &o)
		out = o

	case models.EntityAnnotationFile:
		var o returnEntityAnnotationFile

		err = json.Unmarshal(model.Payload, &o)
		out = o

	default:
		return nil, errors.New("could not adapt annotation type")
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
		var o models.EntityAnnotationTagPayload

		err = json.Unmarshal(payload, &o)
		out = o

	default:
		return nil, fmt.Errorf("invalid annotation type")
	}

	return
}
