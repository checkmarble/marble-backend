package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type Pivot struct {
	Id        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`

	BaseTable    string `json:"base_table"`
	BaseTableId  string `json:"base_table_id"`
	PivotTable   string `json:"pivot_table"`
	PivotTableId string `json:"pivot_table_id"`

	Field   string `json:"field"`
	FieldId string `json:"field_id"`

	PathLinks   []string `json:"path_links"`
	PathLinkIds []string `json:"path_link_ids"`
}

func AdaptPivotDto(pivot models.Pivot) Pivot {
	out := Pivot{
		Id:        pivot.Id,
		CreatedAt: pivot.CreatedAt,

		BaseTable:    pivot.BaseTable,
		BaseTableId:  pivot.BaseTableId,
		PivotTable:   pivot.PivotTable,
		PivotTableId: pivot.PivotTableId,

		Field:   pivot.Field,
		FieldId: pivot.FieldId,

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

// pass either FieldId or PathLinkIds (not both). If PathLinkIds is passed, FieldId will be calculated in the returned object
type CreatePivotInput struct {
	BaseTableId string   `json:"base_table_id" binding:"required"`
	FieldId     *string  `json:"field_id"`
	PathLinkIds []string `json:"path_link_ids"`
}

func AdaptCreatePivotInput(input CreatePivotInput, organizationId string) models.CreatePivotInput {
	out := models.CreatePivotInput{
		OrganizationId: organizationId,
		BaseTableId:    input.BaseTableId,
		FieldId:        input.FieldId,
		PathLinkIds:    make([]string, 0),
	}
	if input.PathLinkIds != nil {
		out.PathLinkIds = input.PathLinkIds
	}
	return out
}
