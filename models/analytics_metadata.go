package models

type AnalyticsType string

const (
	AnalyticsString    AnalyticsType = "string"
	AnalyticsBoolean                 = "bool"
	AnalyticsNumber                  = "number"
	AnalyticsTimestamp               = "timestamp"
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
