package models

import (
	"fmt"
	"marble/marble-backend/pure_utils"

	dynamicstruct "github.com/ompluscator/dynamic-struct"
)

type Payload struct {
	Reader dynamicstruct.Reader
	Table  Table
}

type PayloadForArchive struct {
	TableName string
	Data      map[string]interface{}
}

func (payload Payload) ReadFieldFromPayload(fieldName FieldName) (interface{}, error) {
	field := payload.Reader.GetField(pure_utils.Capitalize(string(fieldName)))
	table := payload.Table
	fields := table.Fields
	fieldFromModel, ok := fields[fieldName]
	if !ok {
		return nil, fmt.Errorf("The field %v is not in table %v schema", fieldName, table.Name)
	}

	switch fieldFromModel.DataType {
	case Bool:
		return field.PointerBool(), nil
	case Int:
		return field.PointerInt(), nil
	case Float:
		return field.PointerFloat64(), nil
	case String:
		return field.PointerString(), nil
	case Timestamp:
		return field.PointerTime(), nil
	default:
		return nil, fmt.Errorf("The field %v has no supported data type", fieldName)
	}
}

type DbFieldReadParams struct {
	TriggerTableName TableName
	Path             []LinkName
	FieldName        FieldName
	DataModel        DataModel
	Payload          Payload
}
