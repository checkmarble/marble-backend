package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/guregu/null/v5"
)

type Pivot struct {
	Id string `json:"id"`

	BaseTable   string `json:"base_table"`
	BaseTableId string `json:"base_table_id"`

	CreatedAt time.Time `json:"created_at"`

	BaseField   null.String `json:"base_field"`
	BaseFieldId null.String `json:"base_field_id"`

	Links   []string `json:"links"`
	LinkIds []string `json:"link_ids"`
}

func AdaptPivotDto(pivot models.Pivot) Pivot {
	return Pivot{
		Id: pivot.Id,

		BaseTable:   pivot.BaseTable,
		BaseTableId: pivot.BaseTableId,

		CreatedAt: pivot.CreatedAt,

		BaseField:   null.StringFromPtr(pivot.BaseField),
		BaseFieldId: null.StringFromPtr(pivot.BaseFieldId),

		Links:   pivot.Links,
		LinkIds: pivot.LinkIds,
	}
}

type CreatePivotInput struct {
	BaseTableId string   `json:"base_table_id" binding:"required"`
	BaseFieldId *string  `json:"base_field_id"`
	LinkIds     []string `json:"link_ids"`
}

func AdaptCreatePivotInput(input CreatePivotInput) models.CreatePivotInput {
	return models.CreatePivotInput{
		BaseTableId: input.BaseTableId,
		BaseFieldId: input.BaseFieldId,
		LinkIds:     input.LinkIds,
	}
}
