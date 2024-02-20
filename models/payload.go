package models

type DbFieldReadParams struct {
	TriggerTableName TableName
	Path             []LinkName
	FieldName        FieldName
	DataModel        DataModel
	ClientObject     ClientObject
}

type ClientObject struct {
	TableName TableName
	Data      map[string]any
}
