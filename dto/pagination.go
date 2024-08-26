package dto

import "github.com/checkmarble/marble-backend/models"

type PaginationAndSortingInput struct {
	OffsetId string              `form:"offset_id"`
	Previous bool                `form:"previous"`
	Next     bool                `form:"next"`
	Sorting  models.SortingField `form:"sorting"`
	Order    models.SortingOrder `form:"order"`
	Limit    int                 `form:"limit" binding:"max=100"`
}

func AdaptPaginationAndSortingInput(input PaginationAndSortingInput) models.PaginationAndSorting {
	return models.PaginationAndSorting{
		OffsetId: input.OffsetId,
		Previous: input.Previous,
		Next:     input.Next,
		Sorting:  input.Sorting,
		Order:    input.Order,
		Limit:    input.Limit,
	}
}

type PaginationDefaults struct {
	Limit  int
	SortBy models.SortingField
	Order  models.SortingOrder
}

func WithPaginationDefaults(pagination PaginationAndSortingInput, defaults PaginationDefaults) PaginationAndSortingInput {
	if pagination.Sorting == "" {
		pagination.Sorting = defaults.SortBy
	}

	if pagination.Order == "" {
		pagination.Order = defaults.Order
	}

	if pagination.Limit == 0 {
		pagination.Limit = defaults.Limit
	}

	return pagination
}

type TotalCount struct {
	Value      int  `json:"value"`
	IsMaxCount bool `json:"is_max_count"`
}

func AdaptTotalCount(totalCount models.TotalCount) TotalCount {
	return TotalCount{
		Value:      totalCount.Value,
		IsMaxCount: totalCount.IsMaxCount,
	}
}
