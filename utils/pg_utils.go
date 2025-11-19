package utils

import (
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Return a []string of columns based on db tag
func ColumnList[T any](prefixes ...string) []string {
	return generateColumnList[T](prefixes, func(prefixes []string, colName string) string {
		return strings.Join(append(prefixes, colName), ".")
	})
}

// This function is the same as above, but will "escape" all columns in
// double-quotes, such as "schema"."table"."column".
func EscapedColumnList[T any](prefixes ...string) []string {
	return generateColumnList[T](prefixes, func(prefixes []string, colName string) string {
		return pgx.Identifier.Sanitize(append(prefixes, colName))
	})
}

// Return a []string of columns based on db tag
func generateColumnList[T any](prefixes []string, generator func([]string, string) string) []string {
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
		result = append(result, generator(prefixes, colName))
	}

	return result
}
