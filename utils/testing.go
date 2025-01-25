package utils

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"

	"github.com/checkmarble/marble-backend/models"
	"github.com/gin-gonic/gin"
	"github.com/go-faker/faker/v4"
)

func FakeStruct[T any]() (T, []any) {
	var object T

	_ = faker.FakeData(&object)

	return object, StructToMockRow(object)
}

func FakeStructs[T any](n int) ([]T, [][]any) {
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

func HandlerTester(req *http.Request, handler func(c *gin.Context)) *httptest.ResponseRecorder {
	ctx := context.TODO()
	ctx = context.WithValue(ctx, ContextKeyCredentials, models.Credentials{})

	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = req.WithContext(ctx)

	handler(ginCtx)

	ginCtx.Writer.WriteHeaderNow()

	return w
}

func JsonTestUnmarshal[T any](r io.Reader) (T, error) {
	var value T

	err := json.NewDecoder(r).Decode(&value)

	return value, err
}
