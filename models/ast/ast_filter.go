package ast

import "slices"

type FilterOperator string

const (
	FILTER_EQUAL             FilterOperator = "="
	FILTER_NOT_EQUAL         FilterOperator = "!="
	FILTER_GREATER           FilterOperator = ">"
	FILTER_GREATER_OR_EQUAL  FilterOperator = ">="
	FILTER_LESSER            FilterOperator = "<"
	FILTER_LESSER_OR_EQUAL   FilterOperator = "<="
	FILTER_IS_IN_LIST        FilterOperator = "IsInList"
	FILTER_IS_NOT_IN_LIST    FilterOperator = "IsNotInList"
	FILTER_IS_EMPTY          FilterOperator = "IsEmpty"
	FILTER_IS_NOT_EMPTY      FilterOperator = "IsNotEmpty"
	FILTER_STARTS_WITH       FilterOperator = "StringStartsWith"
	FILTER_ENDS_WITH         FilterOperator = "StringEndsWith"
	FILTER_UNKNOWN_OPERATION FilterOperator = "FILTER_UNKNOWN_OPERATION"
)

func (op FilterOperator) IsUnary() bool {
	return slices.Contains([]FilterOperator{FILTER_IS_EMPTY, FILTER_IS_NOT_EMPTY}, op)
}

type Filter struct {
	TableName string
	FieldName string
	Operator  FilterOperator
	Value     any
}

var FuncFilterAttributes = FuncAttributes{
	DebugName: "FUNC_FILTER",
	AstName:   "Filter",
	NamedArguments: []string{
		"tableName",
		"fieldName",
		"operator",
		"value",
	},
}
