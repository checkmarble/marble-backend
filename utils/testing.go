package utils

import (
	"reflect"

	"github.com/go-faker/faker/v4"
	"github.com/go-faker/faker/v4/pkg/options"
)

func FakeStruct[T any](opt ...options.OptionFunc) (T, []any) {
	var object T

	_ = faker.FakeData(&object)

	return object, StructToMockRow(object)
}

func FakeStructs[T any](n int, opt ...options.OptionFunc) ([]T, [][]any) {
	objects := make([]T, n)
	rows := make([][]any, n)

	for idx := range n {
		var object T

		_ = faker.FakeData(&object)

		objects[idx] = object
		rows[idx] = StructToMockRow(object)
	}

	return objects, rows
}

func FakeStructRow[T any]() []any {
	_, row := FakeStruct[T]()

	return row
}

func StructToMockRow[T any](object T) []any {
	f := reflect.ValueOf(object)
	t := reflect.TypeOf(object)

	if f.Kind() != reflect.Struct {
		panic("StructToMockRow should only be used on structs")
	}

	slice := make([]any, 0)

	for i := 0; i < f.NumField(); i++ {
		sf := f.Field(i)

		switch sf.Kind() {
		case reflect.Struct:
			switch t.Field(i).Anonymous {
			case true:
				slice = append(slice, StructToMockRow(sf.Interface())...)
			default:
				slice = append(slice, sf.Interface())
			}
		default:
			slice = append(slice, sf.Interface())
		}
	}

	return slice
}
