package models

import (
	"github.com/cockroachdb/errors"
)

type PaginationAndSorting struct {
	OffsetId string
	Sorting  SortingField
	Order    SortingOrder
	Limit    int
}

func NewDefaultPaginationAndSorting(sortColumnName string) PaginationAndSorting {
	return PaginationAndSorting{
		Sorting: SortingFieldFrom(sortColumnName),
		Order:   SortingOrderDesc,
		Limit:   100,
	}
}

type SortingField int

const (
	SortingFieldUnknown SortingField = iota
	SortingFieldCreatedAt
	SortingFieldUpdatedAt
)

func (sf SortingField) String() string {
	switch sf {
	case SortingFieldCreatedAt:
		return "created_at"
	case SortingFieldUpdatedAt:
		return "updated_at"
	default:
		return "unknown"
	}
}

func SortingFieldFrom(s string) SortingField {
	switch s {
	case "created_at":
		return SortingFieldCreatedAt
	case "updated_at":
		return SortingFieldUpdatedAt
	}
	return SortingFieldUnknown
}

type SortingOrder int

const (
	SortingOrderUnknown SortingOrder = iota
	SortingOrderAsc
	SortingOrderDesc
)

func (so SortingOrder) String() string {
	switch so {
	case SortingOrderAsc:
		return "ASC"
	case SortingOrderDesc:
		return "DESC"
	default:
		return "unknown"
	}
}

func SortingOrderFrom(s string) SortingOrder {
	switch s {
	case "ASC":
		return SortingOrderAsc
	case "DESC":
		return SortingOrderDesc
	}
	return SortingOrderUnknown
}

func ValidatePagination(pagination PaginationAndSorting) error {
	if pagination.Order == SortingOrderUnknown {
		return errors.WithDetailf(BadParameterError,
			"invalid pagination: order must be either ASC or DESC, received %s", pagination.Order)
	}
	if pagination.Sorting == SortingFieldUnknown {
		return errors.WithDetailf(BadParameterError,
			"invalid pagination: sorting must be either created_at or updated_at, received %s", pagination.Sorting)
	}
	if pagination.Limit <= 0 {
		return errors.WithDetailf(BadParameterError,
			"invalid pagination: limit must be greater than 0, received %d", pagination.Limit)
	}
	return nil
}

type PaginationDefaults struct {
	Limit  int
	SortBy SortingField
	Order  SortingOrder
}

func WithPaginationDefaults(pagination PaginationAndSorting, defaults PaginationDefaults) PaginationAndSorting {
	if pagination.Sorting == SortingFieldUnknown {
		pagination.Sorting = defaults.SortBy
	}

	if pagination.Order == SortingOrderUnknown {
		pagination.Order = defaults.Order
	}

	if pagination.Limit == 0 {
		pagination.Limit = defaults.Limit
	}

	return pagination
}
