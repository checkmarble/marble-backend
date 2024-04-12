package models

type DbFieldReadParams struct {
	TriggerTableName TableName
	Path             []string
	FieldName        string
	DataModel        DataModel
	ClientObject     ClientObject
}

type ClientObject struct {
	TableName TableName
	Data      map[string]any
}
