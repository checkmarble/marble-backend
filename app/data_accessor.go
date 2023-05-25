package app

import (
	"context"
)

type DataAccessorImpl struct {
	DataModel  DataModel
	Payload    Payload
	repository RepositoryInterface
}

type DbFieldReadParams struct {
	TriggerTableName TableName
	Path             []LinkName
	FieldName        FieldName
	DataModel        DataModel
	Payload          Payload
}

func (d *DataAccessorImpl) GetPayloadField(fieldName string) (interface{}, error) {
	return d.Payload.ReadFieldFromPayload(FieldName(fieldName))
}
func (d *DataAccessorImpl) GetDbField(triggerTableName string, path []string, fieldName string) (interface{}, error) {
	return d.repository.GetDbField(context.TODO(), DbFieldReadParams{
		TriggerTableName: TableName(triggerTableName),
		Path:             toLinkNames(path),
		FieldName:        FieldName(fieldName),
		DataModel:        d.DataModel,
		Payload:          d.Payload,
	})
}
