package pg_repository

import (
	"reflect"
	"strings"
)

// Return a map[string]any to use with Update().SetMap()
//
// Inspired from pgx.RowToStructByName implementation
func updateMapByName[T any](input T) map[string]any {
	result := make(map[string]any)

	inputElemValue := reflect.ValueOf(input)
	if inputElemValue.Kind() != reflect.Ptr {
		inputElemValue = reflect.ValueOf(&input)
	}
	inputElemType := inputElemValue.Elem().Type()

	for i := 0; i < inputElemType.NumField(); i++ {
		sf := inputElemType.Field(i)
		if sf.PkgPath != "" && !sf.Anonymous {
			// Field is unexported, skip it.
			continue
		}
		if sf.Anonymous && sf.Type.Kind() == reflect.Struct {
			// Don't handle anoymous struct embedding
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
		colValue := reflect.Indirect(inputElemValue).FieldByName(sf.Name)
		if colValue.Kind() != reflect.Ptr {
			result[colName] = colValue
		} else if !colValue.IsNil() {
			result[colName] = colValue.Elem()
		}

	}

	return result
}
