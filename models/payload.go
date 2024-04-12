package models

type DbFieldReadParams struct {
	TriggerTableName string
	Path             []string
	FieldName        string
	DataModel        DataModel
	ClientObject     ClientObject
}

type ClientObject struct {
	TableName string
	Data      map[string]any
}
