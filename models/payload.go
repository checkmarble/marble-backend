package models

import (
	"fmt"
	"marble/marble-backend/pure_utils"

	dynamicstruct "github.com/ompluscator/dynamic-struct"
)

type PayloadReader interface {
	ReadFieldFromPayload(fieldName FieldName) (any, error)
	ReadTableName() TableName
}

type DbFieldReadParams struct {
	TriggerTableName TableName
	Path             []LinkName
	FieldName        FieldName
	DataModel        DataModel
	Payload          PayloadReader
}

type Payload struct {
	Reader    dynamicstruct.Reader
	TableName TableName
}

func (payload Payload) ReadFieldFromPayload(fieldName FieldName) (any, error) {
	// output type is string, bool, float64, int64, time.Time, bundled in an "any" interface
	field := payload.Reader.GetField(pure_utils.Capitalize(string(fieldName)))

	return field.Interface(), nil
}

func (payload Payload) ReadTableName() TableName {
	return payload.TableName
}

type ClientObject struct {
	TableName TableName
	Data      map[string]any
}

func (obj ClientObject) ReadFieldFromPayload(fieldName FieldName) (any, error) {
	// output type is string, bool, float64, int64, time.Time, bundled in an "any" interface
	fieldValue, ok := obj.Data[string(fieldName)]
	if !ok {
		return nil, fmt.Errorf("No field with name %s", fieldName)
	}
	return fieldValue, nil
}

func (obj ClientObject) ReadTableName() TableName {
	return obj.TableName
}
