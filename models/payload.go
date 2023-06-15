package models

import (
	"fmt"
	"marble/marble-backend/pure_utils"

	dynamicstruct "github.com/ompluscator/dynamic-struct"
	"gopkg.in/guregu/null.v3"
)

type PayloadReader interface {
	ReadFieldFromPayload(fieldName FieldName) (interface{}, error)
}

type DbFieldReadParams struct {
	TriggerTableName TableName
	Path             []LinkName
	FieldName        FieldName
	DataModel        DataModel
	Payload          PayloadReader
}

type Payload struct {
	Reader dynamicstruct.Reader
	Table  Table
}

func (payload Payload) ReadFieldFromPayload(fieldName FieldName) (interface{}, error) {
	// output type is null.Bool, null.Int, null.Float, null.String, null.Time
	field := payload.Reader.GetField(pure_utils.Capitalize(string(fieldName)))
	table := payload.Table
	fields := table.Fields
	fieldFromModel, ok := fields[fieldName]
	if !ok {
		return nil, fmt.Errorf("The field %v is not in table %v schema", fieldName, table.Name)
	}

	switch fieldFromModel.DataType {
	case Bool:
		return null.BoolFromPtr(field.PointerBool()), nil
	case Int:
		// cast from *int to *int64...
		ptrInt := field.PointerInt()
		ptrInt64 := new(int64)
		if ptrInt != nil {
			*ptrInt64 = int64(*ptrInt)
		}
		return null.IntFromPtr(ptrInt64), nil
	case Float:
		return null.FloatFromPtr(field.PointerFloat64()), nil
	case String:
		return null.StringFromPtr(field.PointerString()), nil
	case Timestamp:
		return null.TimeFromPtr(field.PointerTime()), nil
	default:
		return nil, fmt.Errorf("The field %v has no supported data type", fieldName)
	}
}

type ClientObject struct {
	TableName string
	Data      map[string]interface{}
}

func (obj ClientObject) ReadFieldFromPayload(fieldName FieldName) (interface{}, error) {
	// output type is null.Bool, null.Int, null.Float, null.String, null.Time
	fieldValue, ok := obj.Data[string(fieldName)]
	if !ok {
		return nil, fmt.Errorf("No field with name %s", fieldName)
	}
	return fieldValue, nil
}
