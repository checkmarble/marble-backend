package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type APITag struct {
	Id             string    `json:"id"`
	Name           string    `json:"name"`
	Color          string    `json:"color"`
	OrganizationId uuid.UUID `json:"organization_id"`
	CreatedAt      time.Time `json:"created_at"`
	CasesCount     *int      `json:"cases_count"`
	Target         string    `json:"target"`
}

func AdaptTagDto(t models.Tag) APITag {
	return APITag{
		Id:             t.Id,
		Name:           t.Name,
		Color:          t.Color,
		OrganizationId: t.OrganizationId,
		CreatedAt:      t.CreatedAt,
		CasesCount:     t.CasesCount,
		Target:         string(t.Target),
	}
}

type CreateTagBody struct {
	Target string `json:"target" binding:"omitempty,oneof=case object"`
	Name   string `json:"name" binding:"required"`
	Color  string `json:"color" binding:"required,hexcolor"`
}

type UpdateTagBody struct {
	Name  string `json:"name"`
	Color string `json:"color" binding:"hexcolor"`
}
