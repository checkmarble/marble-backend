package ast

type Aggregator string

const (
	AGGREGATOR_AVG            Aggregator = "AVG"
	AGGREGATOR_COUNT          Aggregator = "COUNT"
	AGGREGATOR_COUNT_DISTINCT Aggregator = "COUNT_DISTINCT"
	AGGREGATOR_MAX            Aggregator = "MAX"
	AGGREGATOR_MIN            Aggregator = "MIN"
	AGGREGATOR_SUM            Aggregator = "SUM"
	AGGREGATOR_UNKNOWN        Aggregator = "Unkown aggregator"
)

var FuncAggregatorAttributes = FuncAttributes{
	DebugName:         "FUNC_AGGREGATOR",
	AstName:           "Aggregator",
	NumberOfArguments: 4,
	NamedArguments:    []string{"tableName", "fieldName", "aggregator", "filters"},
}
