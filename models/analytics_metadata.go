package models

type AnalyticsType string

const (
	AnalyticsString    AnalyticsType = "string"
	AnalyticsBoolean   AnalyticsType = "bool"
	AnalyticsNumber    AnalyticsType = "number"
	AnalyticsTimestamp AnalyticsType = "timestamp"
)

type AnalyticsFieldSource string

const (
	AnalyticsSourceTriggerObject AnalyticsFieldSource = "trigger_object"
	AnalyticsSourceIngestedData  AnalyticsFieldSource = "ingested_data"
)

func AnalyticsTypeFromColumn(colType string) AnalyticsType {
	switch colType {
	case "VARCHAR", "TEXT":
		return AnalyticsString
	case "INTEGER", "DOUBLE", "FLOAT", "BIGINT":
		return AnalyticsNumber
	case "BOOLEAN":
		return AnalyticsBoolean
	case "TIMESTAMP", "TIMESTAMP WITH TIME ZONE":
		return AnalyticsTimestamp
	default:
		return AnalyticsString
	}
}

type AnalyticsFilter struct {
	Name string
	Type string
}
