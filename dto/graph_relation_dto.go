package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
)

type GraphRelation struct {
	Id         uuid.UUID `json:"id"`
	Label      string    `json:"label"`
	LeftType   string    `json:"left_type"`
	LeftField  string    `json:"left_field"`
	RightType  string    `json:"right_type"`
	RightField string    `json:"right_field"`
	CreatedAt  time.Time `json:"created_at"`
}

func AdaptGraphRelationDto(r models.GraphRelation) GraphRelation {
	return GraphRelation{
		Id:         r.Id,
		Label:      r.Label,
		LeftType:   r.LeftType,
		LeftField:  r.LeftField,
		RightType:  r.RightType,
		RightField: r.RightField,
		CreatedAt:  r.CreatedAt,
	}
}

type CreateGraphRelationBody struct {
	Label      string `json:"label" binding:"required"`
	LeftType   string `json:"left_type" binding:"required"`
	LeftField  string `json:"left_field" binding:"required"`
	RightType  string `json:"right_type" binding:"required"`
	RightField string `json:"right_field" binding:"required"`
}

func AdaptCreateGraphRelation(body CreateGraphRelationBody) models.CreateGraphRelation {
	return models.CreateGraphRelation{
		Label:      body.Label,
		LeftType:   body.LeftType,
		LeftField:  body.LeftField,
		RightType:  body.RightType,
		RightField: body.RightField,
	}
}
