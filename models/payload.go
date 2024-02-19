package models

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
