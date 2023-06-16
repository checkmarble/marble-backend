package utils

import (
	"reflect"
	"strings"
)

// Return a []string of columns based on db tag
func ColumnList[T any](prefixes ...string) []string {
	var value T
	var result []string

	inputElemValue := reflect.Indirect(reflect.ValueOf(value))
	inputElemType := inputElemValue.Type()

	for _, sf := range reflect.VisibleFields(inputElemType) {
		if !sf.IsExported() {
			continue
		}
		dbTag, dbTagPresent := sf.Tag.Lookup("db")
		if !dbTagPresent {
			continue
		}
		colName := strings.Split(dbTag, ",")[0]
		if dbTag == "-" {
			// Field is ignored, skip it.
			continue
		}
		colWithPrefixes := strings.Join(append(prefixes, colName), ".")
		result = append(result, colWithPrefixes)
	}

	return result
}
