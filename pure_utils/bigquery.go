package pure_utils

import "cloud.google.com/go/bigquery"

func NullStringFromPtr(ptr *string) bigquery.NullString {
	value := bigquery.NullString{}
	if ptr != nil {
		value.StringVal = *ptr
		value.Valid = true
	}

	return value
}

func NullFloat64FromPtr(ptr *float64) bigquery.NullFloat64 {
	value := bigquery.NullFloat64{}
	if ptr != nil {
		value.Float64 = *ptr
		value.Valid = true
	}

	return value
}
