package models

import (
	"fmt"
)

type DbFieldReadParams struct {
	TriggerTableName TableName
	Path             []LinkName
	FieldName        FieldName
	DataModel        DataModel
	Payload          ClientObject
}

type ClientObject struct {
	TableName TableName
	Data      map[string]any
}

func (obj ClientObject) ReadFieldFromPayload(fieldName FieldName) (any, error) {
	// output type is string, bool, float64, int64, time.Time, bundled in an "any" interface
	fieldValue, ok := obj.Data[string(fieldName)]
	if !ok {
		return nil, fmt.Errorf("no field with name %s", fieldName)
	}
	return fieldValue, nil
}
