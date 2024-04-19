package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/guregu/null/v5"
)

type Pivot struct {
	Id        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`

	BaseTable   string `json:"base_table"`
	BaseTableId string `json:"base_table_id"`

	Field   null.String `json:"field"`
	FieldId null.String `json:"field_id"`

	PathLinks   []string `json:"path_links"`
	PathLinkIds []string `json:"path_link_ids"`
}

func AdaptPivotDto(pivot models.Pivot) Pivot {
	out := Pivot{
		Id:        pivot.Id,
		CreatedAt: pivot.CreatedAt,

		BaseTable:   pivot.BaseTable,
		BaseTableId: pivot.BaseTableId,

		Field:   null.StringFromPtr(pivot.Field),
		FieldId: null.StringFromPtr(pivot.FieldId),

		PathLinks:   make([]string, 0, len(pivot.PathLinks)),
		PathLinkIds: make([]string, 0, len(pivot.PathLinks)),
	}
	if pivot.PathLinks != nil {
		out.PathLinks = pivot.PathLinks
	}
	if pivot.PathLinkIds != nil {
		out.PathLinkIds = pivot.PathLinkIds
	}

	return out
}

type CreatePivotInput struct {
	BaseTableId string   `json:"base_table_id" binding:"required"`
	FieldId     *string  `json:"field_id"`
	PathLinkIds []string `json:"path_link_ids"`
}

func AdaptCreatePivotInput(input CreatePivotInput, organizationId string) models.CreatePivotInput {
	return models.CreatePivotInput{
		OrganizationId: organizationId,
		BaseTableId:    input.BaseTableId,
		FieldId:        input.FieldId,
		PathLinkIds:    input.PathLinkIds,
	}
}
