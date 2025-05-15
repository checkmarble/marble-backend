package pubapi

import "github.com/checkmarble/marble-backend/models"

type PaginationParams struct {
	After string `form:"after" binding:"omitempty,uuid"`
	Order string `form:"order" binding:"omitempty,oneof=ASC DESC"`
	Limit int    `form:"limit" binding:"omitempty,min=1"`
}

func (p PaginationParams) ToModel() models.PaginationAndSorting {
	return models.PaginationAndSorting{
		OffsetId: p.After,
		Order:    models.SortingOrderFrom(p.Order),
		Limit:    p.Limit,
	}
}
