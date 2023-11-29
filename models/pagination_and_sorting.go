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

type PaginationAndSortingInput struct {
	OffsetId string       `form:"offsetId"`
	Previous bool         `form:"previous"`
	Next     bool         `form:"next"`
	Sorting  SortingField `form:"sorting"`
	Order    SortingOrder `form:"order"`
	Limit    int          `form:"limit" binding:"max=100"`
}

func AdaptPaginationAndSortingInput(input PaginationAndSortingInput) PaginationAndSorting {
	return PaginationAndSorting{
		OffsetId: input.OffsetId,
		Previous: input.Previous,
		Next:     input.Next,
		Sorting:  input.Sorting,
		Order:    input.Order,
		Limit:    input.Limit,
	}
}
