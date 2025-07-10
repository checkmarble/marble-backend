// Package pure_utils provides utility functions for various data transformations.
// Functions prefixed with "BQ" are specifically for Google Cloud BigQuery type conversions.
package pure_utils

import "cloud.google.com/go/bigquery"

// BQNullStringFromPtr converts a string pointer to a BigQuery NullString.
// Returns a valid NullString if ptr is not nil, otherwise returns an invalid NullString.
func BQNullStringFromPtr(ptr *string) bigquery.NullString {
	value := bigquery.NullString{}
	if ptr != nil {
		value.StringVal = *ptr
		value.Valid = true
	}
	return value
}

// BQNullFloat64FromPtr converts a float64 pointer to a BigQuery NullFloat64.
// Returns a valid NullFloat64 if ptr is not nil, otherwise returns an invalid NullFloat64.
func BQNullFloat64FromPtr(ptr *float64) bigquery.NullFloat64 {
	value := bigquery.NullFloat64{}
	if ptr != nil {
		value.Float64 = *ptr
		value.Valid = true
	}
	return value
}
