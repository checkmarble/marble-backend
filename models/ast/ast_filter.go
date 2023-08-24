package ast

type FilterOperator string

const (
	FILTER_EQUAL             FilterOperator = "="
	FILTER_NOT_EQUAL         FilterOperator = "!="
	FILTER_GREATER           FilterOperator = ">"
	FILTER_GREATER_OR_EQUAL  FilterOperator = ">="
	FILTER_LESSER            FilterOperator = "<"
	FILTER_LESSER_OR_EQUAL   FilterOperator = "<="
	FILTER_UNKNOWN_OPERATION FilterOperator = "FILTER_UNKNOWN_OPERATION"
)

type Filter struct {
	TableName string
	FieldName string
	Operator  FilterOperator
	Value     any
}

var FuncFilterAttributes = FuncAttributes{
	DebugName:         "FUNC_FILTER",
	AstName:           "Filter",
	NumberOfArguments: 4,
	NamedArguments: []string{
		"tableName",
		"fieldName",
		"operator",
		"value",
	},
}
