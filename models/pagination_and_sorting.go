package models

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
