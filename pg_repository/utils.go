package pg_repository

import (
	"reflect"
	"strings"
)

// Return a map[string]any to use with Update().SetMap()
//
// Inspired from pgx.RowToStructByName implementation
func updateMapByName(input any) map[string]any {
	result := make(map[string]any)

	inputElemValue := reflectValue(input)
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
		colValue := reflect.Indirect(inputElemValue).FieldByName(sf.Name)
		switch colValue.Kind() {
		case reflect.Struct:
			continue
		case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
			if colValue.IsNil() {
				continue
			}
			value := colValue.Elem().Interface()
			if reflect.ValueOf(value).Kind() == reflect.Struct {
				result[colName] = updateMapByName(value)
			} else {
				result[colName] = value
			}
		default:
			result[colName] = colValue.Interface()
		}
	}

	return result
}

func reflectValue(obj interface{}) reflect.Value {
	var val reflect.Value

	if reflect.TypeOf(obj).Kind() == reflect.Ptr {
		val = reflect.ValueOf(obj).Elem()
	} else {
		val = reflect.ValueOf(obj)
	}

	return val
}
