package ast

type Aggregator string

const (
	AGGREGATOR_AVG            Aggregator = "AVG"
	AGGREGATOR_COUNT          Aggregator = "COUNT"
	AGGREGATOR_COUNT_DISTINCT Aggregator = "COUNT_DISTINCT"
	AGGREGATOR_MAX            Aggregator = "MAX"
	AGGREGATOR_MIN            Aggregator = "MIN"
	AGGREGATOR_SUM            Aggregator = "SUM"
	AGGREGATOR_STDDEV         Aggregator = "STDDEV"
	AGGREGATOR_PERCENTILE     Aggregator = "PCTILE"
	AGGREGATOR_MEDIAN         Aggregator = "MEDIAN"
	AGGREGATOR_UNKNOWN        Aggregator = "Unkown aggregator"
)
