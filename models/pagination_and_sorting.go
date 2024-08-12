package models

import "fmt"

type PaginationAndSorting struct {
	OffsetId string
	Sorting  SortingField
	Order    SortingOrder
	Limit    int
	Previous bool
	Next     bool
}

func NewDefaultPaginationAndSorting(sortColumnName string) PaginationAndSorting {
	return PaginationAndSorting{
		Sorting: SortingField(sortColumnName),
		Order:   SortingOrderDesc,
		Limit:   100,
	}
}

type (
	SortingField string
	SortingOrder string
)

const (
	COUNT_ROWS_LIMIT              = 9000
	SortingOrderAsc  SortingOrder = "ASC"
	SortingOrderDesc SortingOrder = "DESC"
)

func ValidatePagination(pagination PaginationAndSorting) error {
	if pagination.OffsetId != "" {
		if pagination.Previous && pagination.Next {
			return fmt.Errorf("invalid pagination: both previous and next are true: %w", BadParameterError)
		}
		if !pagination.Previous && !pagination.Next {
			return fmt.Errorf("invalid pagination: both previous and next are false: %w", BadParameterError)
		}
	}
	if pagination.Order != SortingOrderAsc && pagination.Order != SortingOrderDesc {
		return fmt.Errorf("invalid pagination: order must be either ASC or DESC: %w", BadParameterError)
	}
	return nil
}

func ReverseOrder(order SortingOrder) SortingOrder {
	if order == "DESC" {
		return "ASC"
	}
	return "DESC"
}

type TotalCount struct {
	Value      int
	IsMaxCount bool
}
