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

type SortingField string
type SortingOrder string

const (
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
