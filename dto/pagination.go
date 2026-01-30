package dto

import "github.com/checkmarble/marble-backend/models"

type PaginationAndSorting struct {
	OffsetId string `form:"offset_id"`
	Sorting  string `form:"sorting"`
	Order    string `form:"order"`
	Limit    int    `form:"limit" binding:"max=100"`
}

func AdaptPaginationAndSorting(input PaginationAndSorting) models.PaginationAndSorting {
	return models.PaginationAndSorting{
		OffsetId: input.OffsetId,
		Sorting:  models.SortingFieldFrom(input.Sorting),
		Order:    models.SortingOrderFrom(input.Order),
		Limit:    input.Limit,
	}
}

type Paginated[T any] struct {
	Items       []T  `json:"items"`
	HasNextPage bool `json:"has_next_page"`
}
